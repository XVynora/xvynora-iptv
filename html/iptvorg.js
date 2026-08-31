(() => {
  const state = {
    sources: [],
    channels: [],
    filter: "all",
    query: ""
  };

  const $ = id => document.getElementById(id);

  function escapeHTML(value) {
    return String(value ?? "")
      .replaceAll("&","&amp;")
      .replaceAll("<","&lt;")
      .replaceAll(">","&gt;")
      .replaceAll('"',"&quot;")
      .replaceAll("'","&#039;");
  }

  function toast(message) {
    const el = $("toast");
    el.textContent = message;
    el.classList.add("show");
    clearTimeout(window.__toast);
    window.__toast = setTimeout(() => el.classList.remove("show"),2500);
  }

  async function api(command, data = {}) {
    const body = JSON.stringify({
      cmd: command,
      ...data
    });

    const response = await fetch("/api/", {
      method:"POST",
      headers:{"Content-Type":"application/json"},
      body
    });

    if (!response.ok) {
      throw new Error(`HTTP ${response.status}`);
    }

    return response.json();
  }

  function sourceIcon(id) {
    return {
      uk:"🇬🇧",
      pk:"🇵🇰",
      sports:"⚽"
    }[id] || "📺";
  }

  function renderSources() {
    $("sourceCount").textContent =
      `${state.sources.length} available`;

    $("sources").innerHTML = state.sources.map(source => `
      <article class="xv-source" data-source="${escapeHTML(source.id)}">
        <div class="xv-source-icon">${sourceIcon(source.id)}</div>
        <strong>${escapeHTML(source.name)}</strong>
        <small>${source.imported ? "Imported · ready" : "Available to import"}</small>
      </article>
    `).join("");

    document.querySelectorAll(".xv-source").forEach(card => {
      card.onclick = () => importSource(card.dataset.source);
    });
  }

  function renderChannels() {
    const query = state.query.toLowerCase();

    const filtered = state.channels.filter(channel => {
      const name = String(channel.name || "").toLowerCase();

      if (query && !name.includes(query)) return false;

      if (state.filter === "all") return true;

      if (state.filter === "sky") {
        return name.includes("sky sports") || name.includes("sky sport");
      }

      return channel.source === state.filter;
    });

    $("channelCount").textContent =
      `${filtered.length} channel${filtered.length === 1 ? "" : "s"}`;

    if (!filtered.length) {
      $("channels").innerHTML =
        `<div class="xv-loading">No channels found.</div>`;
      return;
    }

    $("channels").innerHTML = filtered.map(channel => `
      <article class="xv-channel">
        <div>
          ${channel.logo
            ? `<img class="xv-channel-logo" src="${escapeHTML(channel.logo)}" loading="lazy">`
            : `<div class="xv-channel-logo"></div>`}
          <div class="xv-channel-name">${escapeHTML(channel.name)}</div>
          <div class="xv-channel-meta">
            ${escapeHTML(channel.category || channel.source || "Live TV")}
          </div>
        </div>
        <button data-url="${escapeHTML(channel.url || "")}">Watch</button>
      </article>
    `).join("");

    document.querySelectorAll(".xv-channel button").forEach(button => {
      button.onclick = () => {
        if (!button.dataset.url) {
          toast("Channel stream unavailable.");
          return;
        }

        window.open(button.dataset.url,"_blank","noopener");
      };
    });
  }

  async function load() {
    try {
      const result = await api("iptvorg.status");

      state.sources = Array.isArray(result.data)
        ? result.data
        : [];

      renderSources();
    } catch (error) {
      console.error(error);
      toast("Unable to load IPTV-org sources.");
    }

    /*
     * Channel data will be populated from Threadfin's live stream
     * database in Phase 5.
     */
    state.channels = [];
    renderChannels();
  }

  async function importSource(id) {
    const source = state.sources.find(x => x.id === id);

    if (!source) return;

    if (source.imported) {
      toast(`${source.name} is already imported.`);
      return;
    }

    try {
      toast(`Importing ${source.name}…`);

      await api("iptvorg.import",{source:id});

      toast(`${source.name} imported successfully.`);

      await load();
    } catch (error) {
      console.error(error);
      toast(`Unable to import ${source.name}.`);
    }
  }

  document.querySelectorAll("#sourceTabs button").forEach(button => {
    button.onclick = () => {
      document.querySelectorAll("#sourceTabs button")
        .forEach(x => x.classList.remove("active"));

      button.classList.add("active");
      state.filter = button.dataset.source;
      renderChannels();
    };
  });

  $("channelSearch").addEventListener("input", event => {
    state.query = event.target.value;
    renderChannels();
  });

  load();
})();
