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
          body { font-family: ui-sans-serif, system-ui, -apple-system, sans-serif; background: #f9fafb; color: #111827; margin: 0; padding: 0; }
          .container { max-width: 760px; margin: 0 auto; padding: 2rem 1rem; }
          header { margin-bottom: 2rem; }
          h1 { font-size: 1.5rem; font-weight: 700; color: #111827; margin: 0 0 0.25rem; }
          .subtitle { color: #6b7280; font-size: 0.95rem; margin: 0 0 0.75rem; }
          .meta { font-size: 0.8rem; color: #9ca3af; }
          .meta a { color: #4f46e5; text-decoration: none; }
          .meta a:hover { text-decoration: underline; }
          .badge { display: inline-block; background: #eef2ff; color: #4f46e5; border: 1px solid #c7d2fe; border-radius: 4px; font-size: 0.7rem; font-weight: 600; padding: 0 6px; line-height: 1.6; vertical-align: middle; margin-right: 6px; }
          .entries { list-style: none; padding: 0; margin: 0; }
          .entry { background: #fff; border: 1px solid #e5e7eb; border-radius: 12px; padding: 1rem 1.25rem; margin-bottom: 0.75rem; }
          .entry-title { font-weight: 600; color: #111827; font-size: 0.95rem; margin: 0 0 0.3rem; }
          .entry-title a { color: inherit; text-decoration: none; }
          .entry-title a:hover { color: #4f46e5; }
          .entry-summary { color: #6b7280; font-size: 0.85rem; margin: 0 0 0.4rem; }
          .entry-date { color: #9ca3af; font-size: 0.78rem; }
          footer { margin-top: 2.5rem; text-align: center; font-size: 0.8rem; color: #9ca3af; }
          footer a { color: #4f46e5; text-decoration: none; }
          footer a:hover { text-decoration: underline; }
        </style>
      </head>
      <body>
        <div class="container">
          <header>
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
      </body>
    </html>
  </xsl:template>

</xsl:stylesheet>
