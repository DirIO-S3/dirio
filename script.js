(function () {
  const REPO = "DirIO-S3/dirio";
  const API_URL = `https://api.github.com/repos/${REPO}/releases/latest`;

  const OS_LABELS = { linux: "Linux", darwin: "macOS", windows: "Windows" };
  const ARCH_LABELS = { amd64: "x86_64", arm64: "ARM64" };
  const BINARIES = [
    { prefix: "dirio_", label: "dirio (server)" },
    { prefix: "dio_", label: "dio (client)" },
  ];

  function detectOS() {
    const ua = navigator.userAgent || "";
    if (/Windows/i.test(ua)) return "windows";
    if (/Mac OS X|Macintosh/i.test(ua)) return "darwin";
    if (/Linux/i.test(ua)) return "linux";
    return null;
  }

  // Parses "dirio_1.2.3_linux_amd64.tar.gz" -> { binary: "dirio", os: "linux", arch: "amd64" }
  function parseAssetName(name) {
    const match = name.match(/^(dirio|dio)_[^_]+_([a-z0-9]+)_([a-z0-9]+)\.(tar\.gz|zip)$/i);
    if (!match) return null;
    return { binary: match[1], os: match[2].toLowerCase(), arch: match[3].toLowerCase() };
  }

  function buildTable(binaryLabel, assets) {
    const rows = assets
      .map((a) => ({ ...a, ...parseAssetName(a.name) }))
      .filter((a) => a.os && a.arch)
      .sort((a, b) => a.os.localeCompare(b.os) || a.arch.localeCompare(b.arch));

    if (rows.length === 0) return "";

    const body = rows
      .map(
        (a) => `<tr>
          <td>${OS_LABELS[a.os] || a.os}</td>
          <td>${ARCH_LABELS[a.arch] || a.arch}</td>
          <td><a href="${a.browser_download_url}">${a.name}</a></td>
        </tr>`
      )
      .join("");

    return `<table class="downloads">
      <caption>${binaryLabel}</caption>
      <thead><tr><th>OS</th><th>Arch</th><th>Download</th></tr></thead>
      <tbody>${body}</tbody>
    </table>`;
  }

  async function main() {
    const versionBanner = document.getElementById("version-banner");
    const status = document.getElementById("downloads-status");
    const tablesEl = document.getElementById("downloads-tables");
    const primaryBtn = document.getElementById("primary-download");

    let release;
    try {
      const resp = await fetch(API_URL, { headers: { Accept: "application/vnd.github+json" } });
      if (!resp.ok) throw new Error(`GitHub API returned ${resp.status}`);
      release = await resp.json();
    } catch (err) {
      versionBanner.textContent = "Couldn't reach GitHub to check the latest version.";
      status.textContent = "Couldn't load release assets — use the link below instead.";
      return;
    }

    versionBanner.innerHTML = `Latest release: <a href="${release.html_url}">${release.tag_name}</a>`;
    primaryBtn.href = release.html_url;

    const assets = release.assets || [];
    let html = "";
    for (const { prefix, label } of BINARIES) {
      html += buildTable(label, assets.filter((a) => a.name.startsWith(prefix)));
    }

    if (!html) {
      status.textContent = "No downloadable assets found on the latest release — use the link below instead.";
      return;
    }

    status.remove();
    tablesEl.innerHTML = html;

    // Point the primary CTA at a best-effort match for the visitor's OS (server, amd64).
    const os = detectOS();
    if (os) {
      const match = assets
        .map((a) => ({ ...a, ...parseAssetName(a.name) }))
        .find((a) => a.binary === "dirio" && a.os === os && a.arch === "amd64");
      if (match) {
        primaryBtn.href = match.browser_download_url;
        primaryBtn.textContent = `Download for ${OS_LABELS[os]}`;
      }
    }
  }

  main();
})();
