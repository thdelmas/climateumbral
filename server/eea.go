// Client for the EEA Copernicus imperviousness image service — the
// only upstream Tilewhip has. Values: 0-100 = % sealed, 255 = nodata.
// Free, keyless, but external: every game-critical read is either
// tiny (a 3x3 neighbourhood per pledge) or cached (viewport rasters).
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

const eeaExport = "https://image.discomap.eea.europa.eu/arcgis/rest" +
	"/services/GioLandPublic/HRL_ImperviousnessDensity_2018" +
	"/ImageServer/exportImage"

const eeaWater = "https://image.discomap.eea.europa.eu/arcgis/rest" +
	"/services/GioLandPublic/HRL_WaterWetness_2018" +
	"/ImageServer/exportImage"

const waterValue = 254 // merged marker: permanent water or sea

type eeaClient struct {
	http *http.Client

	mu    sync.Mutex
	cache map[string][]byte
	order *list.List // cache keys, oldest first
}

func newEEA() *eeaClient {
	return &eeaClient{
		http:  &http.Client{Timeout: 90 * time.Second},
		cache: map[string][]byte{},
		order: list.New(),
	}
}

const cacheCap = 64

// values fetches a bbox as raw U8 values: imperviousness merged with
// the water mask (WAW class 1 = permanent water, 253 = sea, both ->
// 254), so quays don't read as touching "green" and water paints as
// water. srid is "3857" (viewport rasters) or "3035" (validation).
func (c *eeaClient) values(srid, bbox string, w, h int) ([]byte, error) {
	key := fmt.Sprintf("%s|%s|%dx%d", srid, bbox, w, h)
	c.mu.Lock()
	if v, ok := c.cache[key]; ok {
		c.mu.Unlock()
		return v, nil
	}
	c.mu.Unlock()

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

	c.mu.Lock()
	if _, ok := c.cache[key]; !ok {
		c.cache[key] = img
		c.order.PushBack(key)
		if c.order.Len() > cacheCap {
			old := c.order.Remove(c.order.Front()).(string)
			delete(c.cache, old)
		}
	}
	c.mu.Unlock()
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
	if meta.Href == "" {
		return nil, fmt.Errorf("eea: exportImage returned no href")
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
