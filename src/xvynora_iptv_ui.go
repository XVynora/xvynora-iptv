package src

import (
	"net/http"
	"strings"
)

func XVynoraIPTVUI(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/iptv/" && r.URL.Path != "/iptv" {
		http.NotFound(w, r)
		return
	}

	const page = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>XVynora IPTV</title>
<style>
:root{
 --bg:#050506;
 --panel:#151518;
 --panel2:#202024;
 --text:#f5f5f7;
 --muted:#a1a1a6;
 --accent:#2997ff;
}
*{box-sizing:border-box}
body{
 margin:0;
 background:radial-gradient(circle at top,#202025 0,#09090b 42%,#050506 100%);
 color:var(--text);
 font-family:-apple-system,BlinkMacSystemFont,"SF Pro Display","SF Pro Text",Arial,sans-serif;
 min-height:100vh;
}
header{
 padding:42px 6vw 28px;
 display:flex;
 justify-content:space-between;
 align-items:center;
}
.brand{
 font-size:30px;
 font-weight:700;
 letter-spacing:-1px;
}
.brand span{color:var(--accent)}
.search{
 width:min(420px,45vw);
 background:#242428;
 border:1px solid #36363b;
 color:white;
 border-radius:14px;
 padding:13px 17px;
 font-size:16px;
 outline:none;
}
main{padding:0 6vw 70px}
.hero{
 padding:35px 0 25px;
}
.hero h1{
 font-size:56px;
 letter-spacing:-3px;
 margin:0 0 8px;
}
.hero p{
 color:var(--muted);
 font-size:18px;
}
.tabs{
 display:flex;
 gap:10px;
 overflow:auto;
 padding:12px 0 28px;
}
.tab{
 border:0;
 background:#222226;
 color:#ddd;
 padding:11px 18px;
 border-radius:999px;
 cursor:pointer;
}
.tab.active{
 background:white;
 color:black;
}
.grid{
 display:grid;
 grid-template-columns:repeat(auto-fill,minmax(220px,1fr));
 gap:16px;
}
.card{
 background:linear-gradient(145deg,#1b1b20,#101012);
 border:1px solid #29292e;
 border-radius:18px;
 padding:20px;
 min-height:150px;
 transition:.2s;
 cursor:pointer;
}
.card:hover{
 transform:translateY(-3px) scale(1.01);
 border-color:#4a4a50;
}
.logo{
 height:60px;
 width:100%;
 object-fit:contain;
 margin-bottom:16px;
}
.name{
 font-size:18px;
 font-weight:600;
}
.meta{
 color:var(--muted);
 font-size:13px;
 margin-top:7px;
}
.actions{
 margin-top:30px;
 display:flex;
 gap:10px;
 flex-wrap:wrap;
}
button.action{
 border:0;
 border-radius:12px;
 padding:11px 17px;
 background:#2c2c31;
 color:white;
 cursor:pointer;
}
button.primary{background:var(--accent)}
#status{
 color:var(--muted);
 padding:25px 0;
}
.player{
 position:fixed;
 inset:auto 20px 20px 20px;
 background:#111114;
 border:1px solid #333;
 border-radius:20px;
 padding:15px;
 display:none;
 z-index:10;
 box-shadow:0 20px 70px #000;
}
video{width:100%;max-height:60vh;border-radius:12px;background:#000}
</style>
</head>
<body>
<header>
<div class="brand">XVynora <span>IPTV</span></div>
<input id="search" class="search" placeholder="Search channels…">
</header>
<main>
<section class="hero">
<h1>Watch what you love.</h1>
<p>Live television, sports and channels from IPTV-org.</p>
</section>

<div class="tabs" id="tabs">
<button class="tab active" data-source="uk">🇬🇧 UK</button>
<button class="tab" data-source="pakistan">🇵🇰 Pakistan</button>
<button class="tab" data-source="sports">⚽ Sports</button>
<button class="tab" data-category="Sky Sports">Sky Sports</button>
</div>

<div class="actions">
<button class="action primary" onclick="importSource()">Import source</button>
<button class="action" onclick="updateSource()">Update</button>
<button class="action" onclick="removeSource()">Remove local source</button>
</div>

<div id="status">Loading channels…</div>
<div id="grid" class="grid"></div>
</main>

<div id="player" class="player">
<div id="playing"></div>
<video id="video" controls autoplay></video>
</div>

<script>
let source="uk";
let category="";
let all=[];

const statusEl=document.getElementById("status");
const grid=document.getElementById("grid");
const search=document.getElementById("search");

async function load(){
 statusEl.textContent="Loading…";
 const q=new URLSearchParams({source});
 if(category) q.set("category",category);
 const searchValue=search.value.trim();
 if(searchValue) q.set("search",searchValue);

 const r=await fetch("/xvynora/api/channels?"+q);
 const j=await r.json();

 if(!j.status){
  statusEl.textContent=j.error||"Unable to load channels";
  return;
 }

 all=j.channels||[];
 statusEl.textContent=all.length+" channels";
 render(all);
}

function render(items){
 grid.innerHTML="";
 for(const c of items){
  const el=document.createElement("div");
  el.className="card";
  el.innerHTML=
   (c.logo?'<img class="logo" src="'+escapeHtml(c.logo)+'">':'')+
   '<div class="name">'+escapeHtml(c.name)+'</div>'+
   '<div class="meta">'+escapeHtml(c.category||"TV")+(c.sky_sports?" · Sky Sports":"")+'</div>';
  el.onclick=()=>play(c);
  grid.appendChild(el);
 }
}

function play(c){
 if(!c.stream)return;
 document.getElementById("playing").textContent=c.name;
 const video=document.getElementById("video");
 video.src=c.stream;
 document.getElementById("player").style.display="block";
 video.play().catch(()=>{});
}

async function action(name){
 const r=await fetch("/xvynora/api/"+name+"?source="+encodeURIComponent(source),{method:"POST"});
 const j=await r.json();
 statusEl.textContent=j.status?"Done":"Error: "+(j.error||"unknown");
}

function importSource(){action("import")}
function updateSource(){action("update")}
function removeSource(){action("remove")}

function escapeHtml(v){
 return String(v).replace(/[&<>"']/g,m=>({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;","'":"&#39;"}[m]));
}

document.querySelectorAll(".tab").forEach(t=>{
 t.onclick=()=>{
  document.querySelectorAll(".tab").forEach(x=>x.classList.remove("active"));
  t.classList.add("active");
  source=t.dataset.source||source;
  category=t.dataset.category||"";
  load();
 };
});

let timer;
search.oninput=()=>{
 clearTimeout(timer);
 timer=setTimeout(load,250);
};

load();
</script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Prevent accidental routing of nested paths.
	if strings.TrimSuffix(r.URL.Path, "/") != "/iptv" {
		http.NotFound(w, r)
		return
	}

	_, _ = w.Write([]byte(page))
}
