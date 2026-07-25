// The back office — one page, no build step, no new auth surface.
// The shell is public and empty; everything sensitive arrives only
// after the browser presents the admin token to the same
// token-gated API the operator could curl. The token lives in the
// operator's localStorage, never in a cookie: nothing here is
// CSRF-able and the page can't leak what the API won't serve.
package main

import "net/http"

func handleAdmin(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Robots-Tag", "noindex")
	_, _ = w.Write([]byte(adminHTML))
}

const adminHTML = `<!doctype html>
<html lang="en">
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex">
<title>ClimateUmbral — back office</title>
<style>
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
</style>
<h1>ClimateUmbral — back office</h1>
<p><input id="tok" type="password" placeholder="admin token">
<button id="go">read</button> <span id="err"></span></p>
<h2>Suggestions</h2><div id="sug" class="mut">token required</div>
<h2>Place reports</h2><div id="rep" class="mut">loading…</div>
<h2>Counters</h2><div id="sta" class="mut">loading…</div>
<script>
var $=function(id){return document.getElementById(id)};
var esc=function(s){var d=document.createElement('div');
  d.textContent=s;return d.innerHTML};
function suggestions(){
  var t=$('tok').value.trim();if(!t)return;
  localStorage.setItem('cu-admin',t);$('err').textContent='';
  fetch('/api/suggestions',{headers:{'X-ClimateUmbral-Token':t}})
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
  .catch(function(e){$('err').textContent=e.message+
    ' — wrong token, or none configured on the server'})}
$('go').onclick=suggestions;
$('tok').addEventListener('keydown',function(e){
  if(e.key==='Enter')suggestions()});
var saved=localStorage.getItem('cu-admin');
if(saved){$('tok').value=saved;suggestions()}
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
