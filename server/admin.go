// The back office. Auth is a login-for-session exchange: POST the
// admin token once, receive a 12h HS256 session in an httpOnly
// Secure SameSite=Strict cookie. The browser can't read the
// session (httpOnly kills XSS theft), other sites can't ride it
// (SameSite kills CSRF), and the long-lived admin token crosses
// the wire exactly once per session instead of once per request.
// Login shares the act rate-limiter, so guessing tokens costs a
// visitor the same budget as spamming pledges. With no admin
// token configured the page doesn't exist.
package main

import (
	"encoding/json"
	"io"
	"net/http"
	"time"
)

const sessionCookie = "cu_admin"

// adminReq accepts either credential: the X-ClimateUmbral-Token
// header (curl, scripts) or a valid session cookie (the browser,
// after login).
func (s *server) adminReq(r *http.Request) bool {
	if s.isAdmin(r.Header.Get(tokenHeader)) {
		return true
	}
	if s.adminToken == "" {
		return false
	}
	c, err := r.Cookie(sessionCookie)
	return err == nil &&
		jwtVerify(jwtKey(s.adminToken), c.Value, time.Now())
}

func (s *server) handleAdminLogin(w http.ResponseWriter, r *http.Request) {
	if s.adminToken == "" {
		http.NotFound(w, r)
		return
	}
	var body struct {
		Token string `json:"token"`
	}
	if json.NewDecoder(io.LimitReader(r.Body, 4096)).
		Decode(&body) != nil || !s.isAdmin(body.Token) {
		writeJSON(w, http.StatusForbidden,
			map[string]any{"error": "wrong token"})
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: jwtSign(jwtKey(s.adminToken),
			time.Now()),
		Path: "/", MaxAge: int(jwtTTL.Seconds()),
		HttpOnly: true, Secure: true,
		SameSite: http.SameSiteStrictMode,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) handleAdminLogout(w http.ResponseWriter, _ *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: true,
		SameSite: http.SameSiteStrictMode,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) handleAdmin(w http.ResponseWriter, r *http.Request) {
	if s.adminToken == "" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Robots-Tag", "noindex")
	w.Header().Set("Cache-Control", "no-store")
	if !s.adminReq(r) {
		_, _ = w.Write([]byte(adminLoginHTML))
		return
	}
	_, _ = w.Write([]byte(adminHTML))
}

const adminStyle = `<style>
:root{color-scheme:light dark;
  --bg:#fff;--fg:#1a1a1a;--mut:#667;--line:#ddd;--card:#f6f7f8}
@media(prefers-color-scheme:dark){:root{
  --bg:#101214;--fg:#e8eaec;--mut:#99a;--line:#2a2e33;--card:#1a1e22}}
body{margin:0 auto;max-width:44rem;padding:1.5rem 1rem 4rem;
  background:var(--bg);color:var(--fg);
  font:16px/1.5 system-ui,sans-serif}
h1{font-size:1.2rem}h2{font-size:1rem;margin-top:2rem}
input{font:inherit;padding:.4rem .6rem;border:1px solid var(--line);
  border-radius:6px;background:var(--bg);color:var(--fg);width:16rem}
button{font:inherit;padding:.4rem .9rem;border:1px solid var(--line);
  border-radius:6px;background:var(--card);color:var(--fg);
  cursor:pointer}
.item{background:var(--card);border-radius:8px;padding:.6rem .8rem;
  margin:.5rem 0;white-space:pre-wrap;word-break:break-word}
.item time{display:block;font-size:.8rem;color:var(--mut)}
table{border-collapse:collapse;width:100%}
td,th{text-align:left;padding:.25rem .5rem;
  border-bottom:1px solid var(--line);font-size:.9rem}
td:last-child,th:last-child{text-align:right}
.mut{color:var(--mut)}#err{color:#c33}
</style>`

const adminLoginHTML = `<!doctype html>
<html lang="en">
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex">
<title>ClimateUmbral — sign in</title>
` + adminStyle + `
<h1>ClimateUmbral — back office</h1>
<p><input id="tok" type="password" placeholder="admin token"
  autocomplete="current-password">
<button id="go">sign in</button></p>
<p id="err"></p>
<script>
var go=function(){
  document.getElementById('err').textContent='';
  fetch('/api/admin/login',{method:'POST',
    headers:{'Content-Type':'application/json'},
    body:JSON.stringify({token:
      document.getElementById('tok').value.trim()})})
  .then(function(r){
    if(r.status===204){location.reload();return}
    return r.json().then(function(e){throw new Error(
      e&&e.error?e.error:'HTTP '+r.status)})})
  .catch(function(e){
    document.getElementById('err').textContent=e.message})};
document.getElementById('go').onclick=go;
document.getElementById('tok').addEventListener('keydown',
  function(e){if(e.key==='Enter')go()});
</script>
`

const adminHTML = `<!doctype html>
<html lang="en">
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex">
<title>ClimateUmbral — back office</title>
` + adminStyle + `
<h1>ClimateUmbral — back office
<button id="out" style="float:right">sign out</button></h1>
<p id="err"></p>
<h2>Suggestions</h2><div id="sug" class="mut">loading…</div>
<h2>Place reports</h2><div id="rep" class="mut">loading…</div>
<h2>Counters</h2><div id="sta" class="mut">loading…</div>
<script>
var $=function(id){return document.getElementById(id)};
var esc=function(s){var d=document.createElement('div');
  d.textContent=s;return d.innerHTML};
$('out').onclick=function(){
  fetch('/api/admin/logout',{method:'POST'})
  .then(function(){location.reload()})};
fetch('/api/suggestions')
.then(function(r){if(!r.ok)throw new Error('HTTP '+r.status);
  return r.text()})
.then(function(txt){
  var lines=txt.split('\n').filter(Boolean).reverse();
  if(!lines.length){$('sug').innerHTML='<span class=mut>none yet</span>';return}
  $('sug').innerHTML=lines.map(function(l){
    try{var o=JSON.parse(l);
      return '<div class=item><time>'+esc(o.ts||'')+'</time>'+
        esc(o.text||'')+'</div>'}
    catch(e){return ''}
  }).join('')})
.catch(function(e){$('sug').innerHTML='';
  $('err').textContent='suggestions: '+e.message});
fetch('/api/reports').then(function(r){return r.json()})
.then(function(m){
  var keys=Object.keys(m).sort(function(a,b){return m[b]-m[a]});
  if(!keys.length){$('rep').innerHTML='<span class=mut>none yet</span>';return}
  $('rep').innerHTML='<table><tr><th>place</th><th>issue</th>'+
    '<th>n</th></tr>'+keys.map(function(k){
      var i=k.lastIndexOf('|');
      return '<tr><td>'+esc(k.slice(0,i))+'</td><td>'+
        esc(k.slice(i+1))+'</td><td>'+m[k]+'</td></tr>'
    }).join('')+'</table>'})
.catch(function(){$('rep').textContent='unavailable'});
fetch('/api/stats').then(function(r){return r.json()})
.then(function(m){
  var keys=Object.keys(m).sort();
  $('sta').innerHTML='<table>'+keys.map(function(k){
    return '<tr><td>'+esc(k)+'</td><td>'+m[k]+'</td></tr>'
  }).join('')+'</table>'})
.catch(function(){$('sta').textContent='unavailable'});
</script>
`
