#!/bin/sh
# Certbot writes a marker into the shared /opt-backed work directory.
# No Docker socket or host service access is required by the renewal container.
(
  marker=/var/lib/letsencrypt/nginx-reload
  last=$(stat -c '%Y:%s' "$marker" 2>/dev/null || true)
  while sleep 60; do
    current=$(stat -c '%Y:%s' "$marker" 2>/dev/null || true)
    if [ -n "$current" ] && [ "$current" != "$last" ]; then
      if nginx -t && nginx -s reload; then last=$current; fi
    fi
  done
) &
