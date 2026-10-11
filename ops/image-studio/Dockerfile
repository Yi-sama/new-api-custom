FROM ghcr.io/open-webui/open-webui:v0.11.4-slim

COPY billing-link.js /app/build/static/billing-link.js
COPY billing-bar.css /app/build/static/billing-bar.css
COPY studio-link.js studio-native.js studio-group-picker.js studio-image-picker.js studio-image-size-picker.js studio-image-state.js studio-image-settings.js studio-tool-result.js /app/build/static/
COPY open_webui_bridge.py patch_open_webui.py patch_frontend.py /tmp/

RUN python /tmp/patch_open_webui.py && python /tmp/patch_frontend.py
RUN python -c "from pathlib import Path; p=Path('/app/build/index.html'); s=p.read_text(); marker='<link rel=\"stylesheet\" href=\"/studio-v33/custom/billing-bar.css?v=33\"><script src=\"/studio-v33/custom/studio-link.js?v=33\"></script><script type=\"module\" src=\"/studio-v33/custom/billing-link.js?v=33\"></script>'; p.write_text(s.replace('</head>', marker + '</head>'))"
