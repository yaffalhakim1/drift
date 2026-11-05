/* drift — player wiring.
   Plays one continuous mix, cut into chapters. If the mix file is missing,
   the built-in synth stands in so the page is never silent. */

(function () {
  const mix = window.MIX;
  const chapters = mix.chapters;

  const el = {
    plateHost: document.getElementById("plate"),
    meta: document.getElementById("meta"),
    title: document.getElementById("title"),
    subtitle: document.getElementById("subtitle"),
    kicker: document.getElementById("kickerLabel"),
    play: document.getElementById("play"),
    prev: document.getElementById("prev"),
    next: document.getElementById("next"),
    seek: document.getElementById("seek"),
    seekInput: document.getElementById("seekInput"),
    seekFill: document.getElementById("seekFill"),
    seekKnob: document.getElementById("seekKnob"),
    timeNow: document.getElementById("timeNow"),
    timeLeft: document.getElementById("timeLeft"),
    volume: document.getElementById("vol"),
    queue: document.getElementById("queueList"),
    card: document.querySelector(".card")
  };

  const reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)");
  const plate = window.createPlate(el.plateHost);

  let index = 0;
  let playing = false;
  let dragging = false;
  let pendingSeek = null;
  let hasFile = false;
  // Until the mix file reports its length, the tracklist end is the best
  // guess. Without a file at all, the synth owns the clock instead.
  let duration = mix.end;

  const fmt = (s) => {
    const total = Math.max(0, Math.floor(s));
    return `${Math.floor(total / 60)}:${String(total % 60).padStart(2, "0")}`;
  };

  const chapterEnd = (i) => (i + 1 < chapters.length ? chapters[i + 1][0] : mix.end);
  const chapterLength = (i) => chapterEnd(i) - chapters[i][0];

  function chapterAt(seconds) {
    let lo = 0;
    let hi = chapters.length - 1;
    while (lo < hi) {
      const mid = Math.ceil((lo + hi) / 2);
      if (chapters[mid][0] <= seconds) lo = mid;
      else hi = mid - 1;
    }
    return lo;
  }

  /* ---------- the audio source ---------- */

  const audio = new Audio();
  audio.preload = "metadata";
  audio.src = mix.src;

  const synth = window.LofiEngine();

  audio.addEventListener("loadedmetadata", () => {
    if (Number.isFinite(audio.duration) && audio.duration > 0) {
      duration = audio.duration;
      hasFile = true;
      el.card.classList.add("has-file");
      paintProgress(audio.currentTime || 0, duration, true);
    }
  });

  audio.addEventListener("error", () => {
    // No file: the synth keeps the page playable, and the copy says why.
    hasFile = false;
    el.card.classList.remove("has-file");
    el.card.setAttribute("data-fallback", "");
    el.subtitle.textContent = "No mix file yet. Playing a synthesized stand-in.";
    paintProgress(0, duration, true);
  });

  audio.addEventListener("ended", () => {
    playing = false;
    paintState();
  });

  function position() {
    if (hasFile) return audio.currentTime || 0;
    if (!synth.isReady()) return 0;
    // The stand-in loops one synthesized piece; the progress bar should
    // describe what is actually audible, not the 59 minute tracklist.
    return synth.position();
  }

  /* Progress needs a length that matches the source in use. */
  function liveDuration() {
    return hasFile ? duration : (synth.isReady() ? synth.getDuration() : duration);
  }

  function seekToSeconds(seconds) {
    const target = Math.max(0, Math.min(duration, seconds));
    if (hasFile) {
      audio.currentTime = target;
    } else if (synth.isReady()) {
      synth.seek(target);
    }
    plate.setProgress(duration > 0 ? target / duration : 0);
    paintProgress(target, duration, true);
  }

  /* ---------- rendering ---------- */

  function chapterName(i) {
    const [, artist, title] = chapters[i];
    if (!title) return artist;
    return artist ? `${title} — ${artist}` : title;
  }

  /* The card shows the song title. The artist belongs in the chapter list and
     the screen reader label, not in the headline. */
  function chapterTitle(i) {
    const [, artist, title] = chapters[i];
    return title || artist;
  }

  function buildQueue() {
    el.queue.replaceChildren();

    chapters.forEach((chapter, i) => {
      const li = document.createElement("li");

      const button = document.createElement("button");
      button.type = "button";
      button.className = "row";
      button.dataset.index = String(i);
      button.setAttribute("aria-label", `Play ${chapterName(i)}`);

      const num = document.createElement("span");
      num.className = "row-num";

      const title = document.createElement("span");
      title.className = "row-title";
      title.textContent = chapterTitle(i);

      const dur = document.createElement("span");
      dur.className = "row-dur";
      dur.textContent = fmt(chapterLength(i));

      button.append(num, title, dur);
      li.appendChild(button);
      el.queue.appendChild(li);
    });
  }

  function updateQueue() {
    const rows = el.queue.querySelectorAll(".row");
    rows.forEach((row, i) => {
      const active = i === index;
      const num = row.querySelector(".row-num");

      if (active) row.setAttribute("aria-current", "true");
      else row.removeAttribute("aria-current");

      if (active && playing) {
        const eq = document.createElement("span");
        eq.className = "eq";
        eq.setAttribute("aria-hidden", "true");
        for (let k = 0; k < 3; k++) eq.appendChild(document.createElement("i"));
        num.replaceChildren(eq);
      } else {
        num.textContent = String(i + 1).padStart(2, "0");
      }
    });

    markScrollable();
  }

  function markScrollable() {
    const more = el.queue.scrollHeight > el.queue.clientHeight + 2;
    el.queue.toggleAttribute("data-fade", more);
  }

  function paintMeta() {
    el.title.textContent = chapterTitle(index);

    // The intro has no artist, so repeating the title there reads as a bug.
    const artist = chapterArtist(index);
    el.subtitle.textContent = artist && artist !== chapterTitle(index) ? artist : "";
    el.subtitle.hidden = !el.subtitle.textContent;

    el.seekInput.setAttribute("aria-valuetext",
      `${fmt(chapters[index][0])}, ${chapterName(index)}`);
    plate.setTitle(chapterTitle(index));
    markClipped();
  }

  /* The artist is secondary information: it sits under the title, small and
     muted, so the headline stays the song. */
  function chapterArtist(i) {
    return chapters[i][1] || "";
  }

  function markClipped() {
    const lineHeight = parseFloat(getComputedStyle(el.subtitle).lineHeight) || 21;
    el.subtitle.classList.toggle("is-clipped",
      el.subtitle.scrollHeight > lineHeight + 2);
  }

  function swapMeta() {
    el.meta.classList.add("swap");
    window.setTimeout(() => {
      paintMeta();
      el.meta.classList.remove("swap");
    }, 160);
  }

  function paintProgress(pos, dur, force) {
    const p = dur > 0 ? Math.max(0, Math.min(1, pos / dur)) : 0;
    if (!dragging || force) {
      el.seekFill.style.width = `${(p * 100).toFixed(2)}%`;
      el.seekKnob.style.left = `${(p * 100).toFixed(2)}%`;
      el.seekInput.value = String(Math.round(p * 1000));
    }
    el.timeNow.textContent = fmt(pos);
    el.timeLeft.textContent = `−${fmt(Math.max(0, dur - pos))}`;
  }

  function paintState() {
    el.play.classList.toggle("is-playing", playing);
    el.card.classList.toggle("is-playing", playing);
    el.play.setAttribute("aria-label", playing ? "Pause" : "Play");
    updateQueue();
  }

  /* ---------- transport ---------- */

  function syncIndex(fromSeek) {
    const i = chapterAt(position());
    if (i === index) return;
    index = i;
    if (fromSeek) swapMeta();
    else paintMeta();
    paintState();
  }

  function start(i) {
    index = i;
    playing = true;
    const at = chapters[i][0];
    seekToSeconds(at);

    if (hasFile) {
      audio.play().catch(() => {
        playing = false;
        paintState();
      });
    } else {
      synth.play(window.TRACKS[Math.min(i, window.TRACKS.length - 1)], 0);
    }

    paintMeta();
    paintState();
  }

  function toggle() {
    if (playing) {
      if (hasFile) audio.pause();
      else synth.pause();
      playing = false;
      paintState();
      paintProgress(position(), duration, true);
      return;
    }

    if (!hasFile && !synth.isReady()) {
      start(index);
      return;
    }

    playing = true;
    if (hasFile) {
      audio.play().catch(() => {
        playing = false;
        paintState();
      });
    } else {
      synth.resume();
    }
    paintState();
  }

  function step(direction) {
    const at = position();
    const current = chapterAt(at);

    // Next goes to the following chapter; inside a chapter, previous first
    // rewinds to its start, as a player does before going back a track.
    if (direction > 0) {
      const next = Math.min(chapters.length - 1, current + 1);
      start(next === current ? 0 : next);
      return;
    }

    if (at - chapters[current][0] > 3) {
      start(current);
      return;
    }
    start(Math.max(0, current - 1));
  }

  function nudge(seconds) {
    seekToSeconds(position() + seconds);
  }

  /* ---------- progress loop ---------- */

  let lastFrame = 0;

  function frame(now) {
    const delta = lastFrame ? (now - lastFrame) / 1000 : 0;
    lastFrame = now;

    if (playing && !dragging) {
      const pos = position();
      paintProgress(pos, duration);
      plate.setProgress(duration > 0 ? pos / duration : 0);

      const i = chapterAt(pos);
      if (i !== index) {
        index = i;
        swapMeta();
        paintState();
      }
    }

    if (playing && !reduceMotion.matches) plate.spin(delta);

    window.requestAnimationFrame(frame);
  }

  /* ---------- seek ---------- */

  el.seekInput.addEventListener("input", () => {
    dragging = true;
    el.seek.classList.add("is-dragging");
    const ratio = Number(el.seekInput.value) / 1000;
    el.seekFill.style.width = `${(ratio * 100).toFixed(2)}%`;
    el.seekKnob.style.left = `${(ratio * 100).toFixed(2)}%`;
    el.timeNow.textContent = fmt(ratio * duration);
    el.timeLeft.textContent = `−${fmt(duration - ratio * duration)}`;
    pendingSeek = ratio;
  });

  const endSeek = () => {
    if (!dragging) return;
    dragging = false;
    el.seek.classList.remove("is-dragging");
    if (pendingSeek !== null) {
      seekToSeconds(pendingSeek * duration);
      pendingSeek = null;
      syncIndex(true);
    }
  };

  el.seekInput.addEventListener("change", endSeek);
  el.seekInput.addEventListener("pointerup", endSeek);
  el.seekInput.addEventListener("pointercancel", endSeek);

  /* ---------- controls ---------- */

  el.play.addEventListener("click", toggle);
  el.prev.addEventListener("click", () => step(-1));
  el.next.addEventListener("click", () => step(1));

  el.queue.addEventListener("click", (event) => {
    const row = event.target.closest(".row");
    if (!row) return;
    start(Number(row.dataset.index));
  });

  el.volume.addEventListener("input", () => {
    const v = Number(el.volume.value);
    el.volume.style.setProperty("--v", `${v}%`);
    if (hasFile) audio.volume = v / 100;
    else synth.setVolume(v / 100);
  });

  document.addEventListener("keydown", (event) => {
    if (event.target instanceof HTMLInputElement) return;
    if (event.metaKey || event.ctrlKey || event.altKey) return;

    if (event.code === "Space") {
      event.preventDefault();
      toggle();
    } else if (event.code === "ArrowRight") {
      event.preventDefault();
      if (event.shiftKey) step(1);
      else nudge(5);
    } else if (event.code === "ArrowLeft") {
      event.preventDefault();
      if (event.shiftKey) step(-1);
      else nudge(-5);
    }
  });

  /* ---------- boot ---------- */

  window.initTheme((theme) => plate.setTheme(theme));

  if ("ResizeObserver" in window) {
    new ResizeObserver(markScrollable).observe(el.queue);
  }

  buildQueue();
  paintMeta();
  paintState();
  paintProgress(0, duration, true);
  window.requestAnimationFrame(frame);
})();
