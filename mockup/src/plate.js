/* drift — the plate.
   A record: disc, grooves and a label. It is the one object on the
   page, and it turns while the mix plays.

   Kept deliberately plain: no fake lighting tricks, no shadows on the grooves.
   The disc carries the chrome, the label carries the text. */

window.createPlate = function createPlate(host) {
  const NS = "http://www.w3.org/2000/svg";

  const SIZE = 300;
  const C = SIZE / 2;
  const R_OUT = 138;
  const R_LABEL = 54;
  const R_ARC = R_LABEL + 6;

  const PALETTES = {
    dark: {
      discIn: "#1a1a1f",
      discOut: "#0a0a0d",
      edge: "#ffffff",
      edgeAlpha: 0.1,
      groove: "#ffffff",
      grooveAlpha: 0.05,
      sheenAlpha: 0.07,
      labelTop: "#f4ecdc",
      labelLow: "#ded0b8",
      labelRing: "#c8b89b",
      labelInk: "#2b251f",
      labelDim: "#8b7c64",
      accent: "#e8b487",
      shadow: 0.6
    },
    light: {
      discIn: "#2a2a30",
      discOut: "#131316",
      edge: "#ffffff",
      edgeAlpha: 0.2,
      groove: "#ffffff",
      grooveAlpha: 0.07,
      sheenAlpha: 0.11,
      labelTop: "#fdfaf3",
      labelLow: "#f0e5d1",
      labelRing: "#d7c8ac",
      labelInk: "#2f2820",
      labelDim: "#94866e",
      accent: "#9c5720",
      shadow: 0.32
    }
  };

  const svg = document.createElementNS(NS, "svg");
  svg.setAttribute("viewBox", `0 0 ${SIZE} ${SIZE}`);
  svg.setAttribute("class", "plate-svg");
  svg.setAttribute("role", "img");
  svg.setAttribute("aria-label", "A record that turns while the mix plays");
  host.appendChild(svg);

  let angle = 0;
  let progress = 0;
  let currentTitle = "";
  let theme = "dark";

  let label, titleEl, arc;

  /* Grooves are drawn as a soft band rather than hard lines, so the disc
     reads as a surface, not as a barcode. */
  function grooves(p) {
    let out = "";
    for (let i = 0; i < 20; i++) {
      const t = i / 19;
      const r = (R_LABEL + 10) + (R_OUT - R_LABEL - 14) * t;
      const alpha = (p.grooveAlpha * (1 - t * 0.55)).toFixed(3);
      out += `<circle cx="${C}" cy="${C}" r="${r.toFixed(1)}" fill="none"
                    stroke="${p.groove}" stroke-opacity="${alpha}"
                    stroke-width="${i % 4 === 0 ? 1.1 : 0.7}"/>`;
    }
    return out;
  }

  function markup(p) {
    return `
      <defs>
        <radialGradient id="discGrad" cx="0.4" cy="0.36" r="0.75">
          <stop offset="0" stop-color="${p.discIn}"/>
          <stop offset="0.6" stop-color="${p.discIn}"/>
          <stop offset="1" stop-color="${p.discOut}"/>
        </radialGradient>
        <linearGradient id="sheenGrad" x1="0.12" y1="0" x2="0.8" y2="1">
          <stop offset="0" stop-color="#ffffff" stop-opacity="${p.sheenAlpha}"/>
          <stop offset="0.3" stop-color="#ffffff" stop-opacity="${(p.sheenAlpha * 0.2).toFixed(3)}"/>
          <stop offset="0.52" stop-color="#ffffff" stop-opacity="0"/>
        </linearGradient>
        <linearGradient id="labelGrad" x1="0.25" y1="0" x2="0.8" y2="1">
          <stop offset="0" stop-color="${p.labelTop}"/>
          <stop offset="1" stop-color="${p.labelLow}"/>
        </linearGradient>
        <filter id="plateShadow" x="-25%" y="-25%" width="150%" height="150%">
          <feDropShadow dx="0" dy="8" stdDeviation="12"
                        flood-color="#000" flood-opacity="${p.shadow}"/>
        </filter>
        <clipPath id="discClip"><circle cx="${C}" cy="${C}" r="${R_OUT}"/></clipPath>
      </defs>

      <g filter="url(#plateShadow)">
        <circle cx="${C}" cy="${C}" r="${R_OUT}" fill="url(#discGrad)"/>
      </g>

      <g clip-path="url(#discClip)">
        ${grooves(p)}
        <circle cx="${C}" cy="${C}" r="${R_OUT}" fill="url(#sheenGrad)"/>
      </g>

      <circle cx="${C}" cy="${C}" r="${R_OUT - 0.5}" fill="none"
              stroke="${p.edge}" stroke-opacity="${p.edgeAlpha}"/>

      <!-- the disc turns; the label print stays level, so it stays readable -->
      <g id="plateDisc" transform="translate(${C} ${C})">
        <circle r="${R_LABEL}" fill="url(#labelGrad)"/>
        <circle r="${R_LABEL}" fill="none" stroke="${p.labelRing}" stroke-opacity="0.85"/>
      </g>

      <!-- the progress ring sits on the frame, not the disc, so its 12 o'clock
           start never drifts as the disc turns -->
      <g transform="translate(${C} ${C})">
        <circle r="${R_ARC}" fill="none" stroke="${p.labelRing}" stroke-opacity="0.3"/>
        <circle id="plateArc" r="${R_ARC}" fill="none" stroke="${p.accent}"
                stroke-opacity="0.95" stroke-width="1.6" transform="rotate(-90)"
                stroke-linecap="round" stroke-dasharray="0 999"/>
      </g>

      <g id="platePrint" transform="translate(${C} ${C})">
        <g clip-path="url(#labelClip)">
          <text id="plateTitle" y="0" text-anchor="middle"
                font-family="-apple-system, BlinkMacSystemFont, Segoe UI, system-ui, sans-serif"
                font-size="11" font-weight="600" letter-spacing="-0.15" fill="${p.labelInk}"></text>
        </g>
      </g>
    `;
  }

  let clipEl;

  function build() {
    const p = PALETTES[theme];
    svg.innerHTML = markup(p);

    // A printed label is cut to a chord across its centre, so text never
    // reaches the rim. The clip is rebuilt with the artwork.
    const half = Math.sqrt(R_LABEL * R_LABEL - 16 * 16) - 6;
    clipEl = document.createElementNS(NS, "clipPath");
    clipEl.setAttribute("id", "labelClip");
    const rect = document.createElementNS(NS, "rect");
    rect.setAttribute("x", String(-half));
    rect.setAttribute("y", "-16");
    rect.setAttribute("width", String(half * 2));
    rect.setAttribute("height", "34");
    clipEl.appendChild(rect);
    svg.querySelector("defs").appendChild(clipEl);

    label = svg.querySelector("#plateDisc");
    titleEl = svg.querySelector("#plateTitle");
    arc = svg.querySelector("#plateArc");

    drawPrint();
    drawArc();
    drawAngle();
  }

  /* Fit the printed title to the label: shrink, then shorten. */
  function fitTitle(value) {
    const max = 2 * (Math.sqrt(R_LABEL * R_LABEL - 16 * 16) - 8);
    let size = 11;
    titleEl.setAttribute("font-size", String(size));
    titleEl.textContent = value;

    while (size > 7.5 && titleEl.getComputedTextLength() > max) {
      size -= 0.25;
      titleEl.setAttribute("font-size", String(size));
    }
    if (titleEl.getComputedTextLength() <= max) return;

    let lo = 0;
    let hi = value.length;
    while (lo < hi) {
      const mid = Math.ceil((lo + hi) / 2);
      titleEl.textContent = `${value.slice(0, mid).trimEnd()}…`;
      if (titleEl.getComputedTextLength() <= max) lo = mid;
      else hi = mid - 1;
    }
    titleEl.textContent = `${value.slice(0, lo).trimEnd()}…`;
  }

  function drawPrint() {
    fitTitle(currentTitle);
  }
  function drawArc() {
    if (!arc) return;
    const len = 2 * Math.PI * R_ARC;
    arc.setAttribute("stroke-dasharray", `${(len * progress).toFixed(2)} ${len.toFixed(2)}`);
  }

  function drawAngle() {
    label.setAttribute("transform", `translate(${C} ${C}) rotate(${angle.toFixed(2)})`);
  }

  function setProgress(p) {
    progress = Math.max(0, Math.min(1, p));
    drawArc();
  }

  /* A record at 33 rpm is a blur. Slower reads as playing and stays calm. */
  function spin(deltaSeconds) {
    angle = (angle + deltaSeconds * 30) % 360;
    drawAngle();
  }

  /* The plate prints the song title only. The artist is a detail for the
     main card, not for a 54 px label. */
  function setTitle(title) {
    currentTitle = title || "";
    drawPrint();
  }

  function setTheme(next) {
    if (next !== "light" && next !== "dark") return;
    if (next === theme) return;
    theme = next;
    build();
  }

  build();

  return { setProgress, spin, setTitle, setTheme };
};
