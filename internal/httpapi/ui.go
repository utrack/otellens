package httpapi

import "net/http"

func (h *Handler) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, "/ui", http.StatusTemporaryRedirect)
}

func (h *Handler) handleUI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(uiPage))
}

const uiPage = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>otellens live capture</title>
  <style>
    :root {
      --bg: #0b1220;
      --panel: #121b2f;
      --panel-2: #18243f;
      --accent: #23d7b4;
      --ink: #e7edf8;
      --muted: #9fb0ce;
      --danger: #f17272;
      --warn: #ffd166;
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      font-family: "JetBrains Mono", "Fira Mono", "IBM Plex Mono", monospace;
      color: var(--ink);
      background: radial-gradient(circle at 10% 10%, #1b2950 0%, var(--bg) 42%);
      min-height: 100vh;
    }
    .wrap { max-width: 1200px; margin: 0 auto; padding: 20px; }
    .title { margin: 0 0 14px; font-size: 22px; color: var(--accent); letter-spacing: 0.3px; }
    .grid { display: grid; gap: 14px; grid-template-columns: 340px 1fr; }
    .card {
      background: linear-gradient(170deg, var(--panel) 0%, var(--panel-2) 100%);
      border: 1px solid #2c3b61;
      border-radius: 12px;
      padding: 14px;
    }
    .row { margin-bottom: 10px; }
    label { display: block; color: var(--muted); margin-bottom: 6px; font-size: 12px; }
    input, textarea {
      width: 100%;
      background: #0e1627;
      border: 1px solid #33476f;
      color: var(--ink);
      border-radius: 8px;
      padding: 8px;
      font: inherit;
      font-size: 12px;
    }
    textarea { min-height: 64px; resize: vertical; }
    .signals { display: flex; gap: 8px; flex-wrap: wrap; }
    .chip {
      display: inline-flex;
      align-items: center;
      gap: 6px;
      background: #0e1627;
      border: 1px solid #33476f;
      border-radius: 999px;
      padding: 5px 10px;
      font-size: 12px;
    }
    .btns { display: flex; gap: 8px; }
    button {
      border: 1px solid #31589f;
      background: #1f3664;
      color: #eef5ff;
      border-radius: 8px;
      padding: 8px 10px;
      font: inherit;
      font-size: 12px;
      cursor: pointer;
    }
    button.primary { background: #136c5a; border-color: #1ea287; }
    button.danger { background: #672626; border-color: #a94141; }
    button:disabled { opacity: 0.45; cursor: not-allowed; }
    .status { font-size: 12px; margin-top: 8px; color: var(--muted); }
    .status.ok { color: var(--accent); }
    .status.err { color: var(--danger); }
    .events { height: calc(100vh - 120px); overflow: auto; display: grid; gap: 8px; }
    .event { background: #0e1627; border: 1px solid #31415f; border-radius: 10px; padding: 10px; }
    .event.end { border-color: #846a30; }
    .event pre { margin: 0; white-space: pre-wrap; word-break: break-word; font-size: 12px; color: #dce8ff; }
    .muted { color: var(--muted); font-size: 11px; }
    @media (max-width: 980px) {
      .grid { grid-template-columns: 1fr; }
      .events { height: auto; max-height: 60vh; }
    }
  </style>
</head>
<body>
  <div class="wrap">
    <h1 class="title">otellens / live capture UI</h1>
    <div class="grid">
      <section class="card">
        <form id="capture-form">
          <div class="row">
            <label>Signals</label>
            <div class="signals">
              <label class="chip"><input type="checkbox" name="signals" value="metrics" checked /> metrics</label>
              <label class="chip"><input type="checkbox" name="signals" value="traces" /> traces</label>
              <label class="chip"><input type="checkbox" name="signals" value="logs" /> logs</label>
            </div>
          </div>

          <div class="row">
            <label for="metric_names">metric_names (comma-separated, prefix with ! for NOT)</label>
            <input id="metric_names" placeholder="http.server.request.duration,!foo,!bar" />
          </div>

          <div class="row">
            <label for="span_names">span_names (comma-separated, prefix with ! for NOT)</label>
            <input id="span_names" placeholder="GET /v1/orders,!POST /health" />
          </div>

          <div class="row">
            <label for="attribute_names">attribute_names (comma-separated keys, prefix with ! for NOT)</label>
            <input id="attribute_names" placeholder="client_name,!blocked" />
          </div>

          <div class="row">
            <label for="resource_attributes">resource_attributes (key=value per line)</label>
            <textarea id="resource_attributes" placeholder="service.name=checkout\ndeployment.environment.name=prod"></textarea>
          </div>

          <div class="row">
            <label for="log_body_contains">log_body_contains</label>
            <input id="log_body_contains" placeholder="timeout" />
          </div>

          <div class="row">
            <label for="min_severity_number">min_severity_number</label>
            <input id="min_severity_number" type="number" min="0" value="0" />
          </div>

          <div class="row">
            <label for="max_batches">max_batches</label>
            <input id="max_batches" type="number" min="1" value="15" required />
          </div>

          <div class="row">
            <label for="timeout_seconds">timeout_seconds</label>
            <input id="timeout_seconds" type="number" min="0" value="30" required />
          </div>

          <div class="row">
            <label class="chip"><input id="verbose_metrics" type="checkbox" /> verbose_metrics (include histogram bucket_counts/explicit_bounds)</label>
          </div>

          <div class="btns">
            <button class="primary" id="start" type="submit">start stream</button>
            <button class="danger" id="stop" type="button" disabled>stop</button>
            <button id="clear" type="button">clear</button>
          </div>
          <div id="status" class="status">idle</div>
        </form>
      </section>

      <section class="card">
        <div class="muted">events</div>
        <div id="events" class="events"></div>
      </section>
    </div>
  </div>

  <script>
    const form = document.getElementById('capture-form');
    const statusEl = document.getElementById('status');
    const eventsEl = document.getElementById('events');
    const startBtn = document.getElementById('start');
    const stopBtn = document.getElementById('stop');
    const clearBtn = document.getElementById('clear');
    let controller = null;

    function parseCSV(value) {
      return value
        .split(',')
        .map((v) => v.trim())
        .filter(Boolean);
    }

    function parseResourceAttributes(value) {
      const out = {};
      value
        .split('\n')
        .map((line) => line.trim())
        .filter(Boolean)
        .forEach((line) => {
          const idx = line.indexOf('=');
          if (idx <= 0) return;
          const key = line.slice(0, idx).trim();
          const val = line.slice(idx + 1).trim();
          if (key) out[key] = val;
        });
      return out;
    }

    function setStatus(text, cls) {
      statusEl.textContent = text;
      statusEl.className = 'status ' + (cls || '');
    }

    function addEvent(event) {
      const wrap = document.createElement('div');
      wrap.className = 'event' + (event.type === 'end' ? ' end' : '');
      const pre = document.createElement('pre');
      pre.textContent = JSON.stringify(event, null, 2);
      wrap.appendChild(pre);
      eventsEl.prepend(wrap);
    }

    function setRunning(running) {
      startBtn.disabled = running;
      stopBtn.disabled = !running;
    }

    async function streamCapture(payload) {
      controller = new AbortController();
      setRunning(true);
      setStatus('connecting...', '');

      try {
        const response = await fetch('/v1/capture/stream', {
          method: 'POST',
          headers: { 'content-type': 'application/json' },
          body: JSON.stringify(payload),
          signal: controller.signal,
        });

        if (!response.ok) {
          const text = await response.text();
          throw new Error('HTTP ' + response.status + ': ' + text);
        }

        setStatus('streaming', 'ok');

        const reader = response.body.getReader();
        const decoder = new TextDecoder();
        let buf = '';

        while (true) {
          const { done, value } = await reader.read();
          if (done) break;

          buf += decoder.decode(value, { stream: true });
          let nl = buf.indexOf('\n');
          while (nl >= 0) {
            const line = buf.slice(0, nl).trim();
            buf = buf.slice(nl + 1);
            if (line) {
              try {
                addEvent(JSON.parse(line));
              } catch (e) {
                addEvent({ type: 'parse_error', line });
              }
            }
            nl = buf.indexOf('\n');
          }
        }

        if (buf.trim()) {
          try {
            addEvent(JSON.parse(buf.trim()));
          } catch (e) {
            addEvent({ type: 'parse_error', line: buf.trim() });
          }
        }

        setStatus('stream ended', 'ok');
      } catch (err) {
        if (err.name === 'AbortError') {
          setStatus('stopped by user', 'warn');
        } else {
          setStatus(err.message || String(err), 'err');
        }
      } finally {
        controller = null;
        setRunning(false);
      }
    }

    form.addEventListener('submit', (ev) => {
      ev.preventDefault();
      if (controller) return;

      const signals = Array.from(document.querySelectorAll('input[name="signals"]:checked')).map((x) => x.value);

      const payload = {
        signals,
        metric_names: parseCSV(document.getElementById('metric_names').value),
        span_names: parseCSV(document.getElementById('span_names').value),
        attribute_names: parseCSV(document.getElementById('attribute_names').value),
        resource_attributes: parseResourceAttributes(document.getElementById('resource_attributes').value),
        log_body_contains: document.getElementById('log_body_contains').value.trim(),
        min_severity_number: Number(document.getElementById('min_severity_number').value || 0),
        verbose_metrics: document.getElementById('verbose_metrics').checked,
        max_batches: Number(document.getElementById('max_batches').value || 15),
        timeout_seconds: Number(document.getElementById('timeout_seconds').value || 30),
      };

      streamCapture(payload);
    });

    stopBtn.addEventListener('click', () => {
      if (controller) controller.abort();
    });

    clearBtn.addEventListener('click', () => {
      eventsEl.innerHTML = '';
    });
  </script>
</body>
</html>`
