<?xml version="1.0" encoding="UTF-8"?>
<xsl:stylesheet version="1.0"
  xmlns:xsl="http://www.w3.org/1999/XSL/Transform"
  xmlns:atom="http://www.w3.org/2005/Atom"
  exclude-result-prefixes="atom">

  <xsl:output method="html" version="1.0" encoding="UTF-8" indent="yes"/>

  <xsl:template match="/">
    <html lang="en">
      <head>
        <meta charset="UTF-8"/>
        <meta name="viewport" content="width=device-width, initial-scale=1.0"/>
        <title><xsl:value-of select="atom:feed/atom:title"/></title>
        <style>
          :root {
            --bg: #f9fafb; --fg: #111827; --fg-muted: #6b7280; --fg-subtle: #9ca3af;
            --card-bg: #fff; --card-border: #e5e7eb;
            --accent: #4f46e5; --badge-bg: #eef2ff; --badge-border: #c7d2fe;
            --toggle-bg: #e5e7eb; --toggle-fg: #374151;
          }
          [data-theme="dark"] {
            --bg: #0f172a; --fg: #f1f5f9; --fg-muted: #94a3b8; --fg-subtle: #64748b;
            --card-bg: #1e293b; --card-border: #334155;
            --accent: #818cf8; --badge-bg: #1e1b4b; --badge-border: #3730a3;
            --toggle-bg: #334155; --toggle-fg: #e2e8f0;
          }
          @media (prefers-color-scheme: dark) {
            :root:not([data-theme="light"]) {
              --bg: #0f172a; --fg: #f1f5f9; --fg-muted: #94a3b8; --fg-subtle: #64748b;
              --card-bg: #1e293b; --card-border: #334155;
              --accent: #818cf8; --badge-bg: #1e1b4b; --badge-border: #3730a3;
              --toggle-bg: #334155; --toggle-fg: #e2e8f0;
            }
          }
          body { font-family: ui-sans-serif, system-ui, -apple-system, sans-serif; background: var(--bg); color: var(--fg); margin: 0; padding: 0; transition: background 0.2s, color 0.2s; }
          .container { max-width: 760px; margin: 0 auto; padding: 2rem 1rem; }
          header { margin-bottom: 2rem; }
          .header-row { display: flex; align-items: flex-start; justify-content: space-between; gap: 1rem; }
          .header-content { flex: 1; }
          h1 { font-size: 1.5rem; font-weight: 700; color: var(--fg); margin: 0 0 0.25rem; }
          .subtitle { color: var(--fg-muted); font-size: 0.95rem; margin: 0 0 0.75rem; }
          .meta { font-size: 0.8rem; color: var(--fg-subtle); }
          .meta a { color: var(--accent); text-decoration: none; }
          .meta a:hover { text-decoration: underline; }
          .badge { display: inline-block; background: var(--badge-bg); color: var(--accent); border: 1px solid var(--badge-border); border-radius: 4px; font-size: 0.7rem; font-weight: 600; padding: 0 6px; line-height: 1.6; vertical-align: middle; margin-right: 6px; }
          .entries { list-style: none; padding: 0; margin: 0; }
          .entry { background: var(--card-bg); border: 1px solid var(--card-border); border-radius: 12px; padding: 1rem 1.25rem; margin-bottom: 0.75rem; }
          .entry-title { font-weight: 600; color: var(--fg); font-size: 0.95rem; margin: 0 0 0.3rem; }
          .entry-title a { color: inherit; text-decoration: none; }
          .entry-title a:hover { color: var(--accent); }
          .entry-summary { color: var(--fg-muted); font-size: 0.85rem; margin: 0 0 0.4rem; }
          .entry-date { color: var(--fg-subtle); font-size: 0.78rem; }
          footer { margin-top: 2.5rem; text-align: center; font-size: 0.8rem; color: var(--fg-subtle); }
          footer a { color: var(--accent); text-decoration: none; }
          footer a:hover { text-decoration: underline; }
          .theme-toggle { flex-shrink: 0; background: var(--toggle-bg); color: var(--toggle-fg); border: 1px solid var(--card-border); border-radius: 8px; padding: 0.4rem 0.75rem; font-size: 0.8rem; cursor: pointer; font-family: inherit; transition: background 0.2s, color 0.2s; white-space: nowrap; }
          .theme-toggle:hover { opacity: 0.8; }
        </style>
      </head>
      <body>
        <script>
          (function() {
            var stored = localStorage.getItem('theme');
            if (stored) document.documentElement.setAttribute('data-theme', stored);
          })();
        </script>
        <div class="container">
          <header>
            <div class="header-row">
              <div class="header-content">
                <h1>
                  <span class="badge">Atom Feed</span>
                  <xsl:value-of select="atom:feed/atom:title"/>
                </h1>
                <p class="subtitle"><xsl:value-of select="atom:feed/atom:subtitle"/></p>
                <p class="meta">
                  This is a live Atom feed. Subscribe in your feed reader, or
                  <a href="/">view the full site</a>.
                  &#160;·&#160;
                  <a href="/feed.xml">Raw XML</a>
                </p>
              </div>
              <button class="theme-toggle" id="theme-toggle" onclick="toggleTheme()">&#9790; Dark</button>
            </div>
          </header>

          <ul class="entries">
            <xsl:for-each select="atom:feed/atom:entry">
              <li class="entry">
                <div class="entry-title">
                  <a>
                    <xsl:attribute name="href">
                      <xsl:value-of select="atom:link/@href"/>
                    </xsl:attribute>
                    <xsl:value-of select="atom:title"/>
                  </a>
                </div>
                <div class="entry-summary"><xsl:value-of select="atom:summary"/></div>
                <div class="entry-date"><xsl:value-of select="atom:updated"/></div>
              </li>
            </xsl:for-each>
          </ul>

          <footer>
            <a href="/">wasitdown.dev</a> &#160;·&#160;
            <a href="/feed.xml">Subscribe</a>
          </footer>
        </div>
        <script>
          function isDark() {
            var theme = document.documentElement.getAttribute('data-theme');
            if (theme === 'dark') return true;
            if (theme === 'light') return false;
            return window.matchMedia('(prefers-color-scheme: dark)').matches;
          }
          function updateButton() {
            var btn = document.getElementById('theme-toggle');
            if (btn) btn.textContent = isDark() ? '\u2600 Light' : '\u263A Dark';
          }
          function toggleTheme() {
            var next = isDark() ? 'light' : 'dark';
            document.documentElement.setAttribute('data-theme', next);
            localStorage.setItem('theme', next);
            updateButton();
          }
          updateButton();
          window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', updateButton);
        </script>
      </body>
    </html>
  </xsl:template>

</xsl:stylesheet>
