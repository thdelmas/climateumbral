// Client for the EEA Copernicus imperviousness image service — the
// only upstream Tilewhip has. Values: 0-100 = % sealed, 255 = nodata.
// Free, keyless, but external: every game-critical read is either
// tiny (a 3x3 neighbourhood per pledge) or cached (viewport rasters),
// and concurrent misses for the same key share one upstream fetch.
package main

import (
	"container/list"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"
)

const eeaHost = "image.discomap.eea.europa.eu"

const eeaExport = "https://" + eeaHost + "/arcgis/rest" +
	"/services/GioLandPublic/HRL_ImperviousnessDensity_2018" +
	"/ImageServer/exportImage"

const eeaWater = "https://" + eeaHost + "/arcgis/rest" +
	"/services/GioLandPublic/HRL_WaterWetness_2018" +
	"/ImageServer/exportImage"

const waterValue = 254 // merged marker: permanent water or sea

type flight struct {
	done chan struct{}
	img  []byte
	err  error
}

type eeaClient struct {
	http *http.Client

	mu       sync.Mutex
	cache    map[string][]byte
	order    *list.List // cache keys, oldest first
	inflight map[string]*flight
}

func newEEA() *eeaClient {
	return &eeaClient{
		http:     &http.Client{Timeout: 90 * time.Second},
		cache:    map[string][]byte{},
		order:    list.New(),
		inflight: map[string]*flight{},
	}
}

const cacheCap = 64

// values fetches a bbox as raw U8 values: imperviousness merged with
// the water mask (WAW class 1 = permanent water, 253 = sea, both ->
// 254), so quays don't read as touching "green" and water paints as
// water. srid is "3857" (viewport rasters) or "3035" (validation).
// One upstream fetch per key at a time: latecomers wait for the
// flight already in the air instead of launching their own.
func (c *eeaClient) values(srid, bbox string, w, h int) ([]byte, error) {
	key := fmt.Sprintf("%s|%s|%dx%d", srid, bbox, w, h)
	c.mu.Lock()
	if v, ok := c.cache[key]; ok {
		c.mu.Unlock()
		return v, nil
	}
	if f, ok := c.inflight[key]; ok {
		c.mu.Unlock()
		<-f.done
		return f.img, f.err
	}
	f := &flight{done: make(chan struct{})}
	c.inflight[key] = f
	c.mu.Unlock()

	f.img, f.err = c.fetchMerged(srid, bbox, w, h)

	c.mu.Lock()
	delete(c.inflight, key)
	if f.err == nil {
		if _, ok := c.cache[key]; !ok {
			c.cache[key] = f.img
			c.order.PushBack(key)
			if c.order.Len() > cacheCap {
				old := c.order.Remove(c.order.Front()).(string)
				delete(c.cache, old)
			}
		}
	}
	c.mu.Unlock()
	close(f.done)
	return f.img, f.err
}

// fetchMerged pulls the imperviousness and water layers in parallel
// and merges the water mask in.
func (c *eeaClient) fetchMerged(
	srid, bbox string, w, h int,
) ([]byte, error) {
	var img, waw []byte
	var errImg, errWaw error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		img, errImg = c.exportLayer(eeaExport, srid, bbox, w, h)
	}()
	go func() {
		defer wg.Done()
		waw, errWaw = c.exportLayer(eeaWater, srid, bbox, w, h)
	}()
	wg.Wait()
	if errImg != nil {
		return nil, errImg
	}
	// water degrades gracefully: without the mask the game still
	// works, water just reads as 0% sealed
	if errWaw != nil {
		log.Printf("eea: water mask unavailable: %v", errWaw)
	} else {
		for i := range img {
			if waw[i] == 1 || waw[i] == 253 {
				img[i] = waterValue
			}
		}
	}
	return img, nil
}

// neighborhood returns the 3x3 values around continent pixel (pe, pn),
// row 0 = north, so index 4 is the pixel itself. Water-merged, so
// client candidates and server validation see the same values.
func (c *eeaClient) neighborhood(pe, pn int) ([]byte, error) {
	bbox := fmt.Sprintf("%d,%d,%d,%d",
		pe*10-10, pn*10-10, pe*10+20, pn*10+20)
	return c.values("3035", bbox, 3, 3)
}

// exportLayer fetches one image-service layer as raw U8 values.
func (c *eeaClient) exportLayer(
	base, srid, bbox string, w, h int,
) ([]byte, error) {
	q := url.Values{
		"bbox":      {bbox},
		"bboxSR":    {srid},
		"imageSR":   {srid},
		"size":      {fmt.Sprintf("%d,%d", w, h)},
		"format":    {"tiff"},
		"pixelType": {"U8"},
		"f":         {"json"},
	}
	var meta struct {
		Href string `json:"href"`
	}
	if err := c.getJSON(base+"?"+q.Encode(), &meta); err != nil {
		return nil, err
	}
	// the href comes from upstream JSON: never follow it off the EEA
	// host, or a poisoned response steers our fetches anywhere
	hu, err := url.Parse(meta.Href)
	if err != nil || hu.Scheme != "https" || hu.Host != eeaHost {
		return nil, fmt.Errorf("eea: exportImage href not on %s",
			eeaHost)
	}
	tif, err := c.getBytes(meta.Href)
	if err != nil {
		return nil, err
	}
	return parseTIFF(tif, w, h)
}

func (c *eeaClient) getJSON(u string, v any) error {
	raw, err := c.getBytes(u)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, v)
}

func (c *eeaClient) getBytes(u string) ([]byte, error) {
	res, err := c.http.Get(u)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("eea: %s -> %s", u[:min(len(u), 80)],
			res.Status)
	}
	return io.ReadAll(io.LimitReader(res.Body, 32<<20))
}
