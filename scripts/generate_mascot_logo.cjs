const fs = require('fs');
const path = require('path');
const { Resvg } = require('B:/workgit/node_modules/@resvg/resvg-js');

function buildLogoSvg() {
  return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1024 1024" width="1024" height="1024">
  <defs>
    <clipPath id="squircle-clip">
      <rect x="24" y="24" width="976" height="976" rx="220" />
    </clipPath>

    <!-- Electric Cyan Telemetry Gradients -->
    <linearGradient id="cyan-glow" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#00f5ff"/>
      <stop offset="100%" stop-color="#0284c7"/>
    </linearGradient>

    <linearGradient id="cyan-iris" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#38bdf8"/>
      <stop offset="100%" stop-color="#00f5ff"/>
    </linearGradient>

    <!-- Amber Alert Beak & Sensor Accents -->
    <linearGradient id="amber-accent" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#fbbf24"/>
      <stop offset="100%" stop-color="#ea580c"/>
    </linearGradient>

    <!-- Obsidian & Titanium Low-Poly Shading -->
    <linearGradient id="f-highlight" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#94a3b8"/>
      <stop offset="100%" stop-color="#64748b"/>
    </linearGradient>

    <linearGradient id="f-light" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#64748b"/>
      <stop offset="100%" stop-color="#475569"/>
    </linearGradient>

    <linearGradient id="f-mid" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#334155"/>
      <stop offset="100%" stop-color="#1e293b"/>
    </linearGradient>

    <linearGradient id="f-dark" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#1e293b"/>
      <stop offset="100%" stop-color="#0f172a"/>
    </linearGradient>

    <linearGradient id="f-obsidian" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#0f172a"/>
      <stop offset="100%" stop-color="#060911"/>
    </linearGradient>

    <filter id="subtle-shadow" x="-15%" y="-15%" width="130%" height="130%">
      <feDropShadow dx="0" dy="16" stdDeviation="22" flood-color="#000000" flood-opacity="0.20" />
    </filter>

    <filter id="optic-glow" x="-20%" y="-20%" width="140%" height="140%">
      <feGaussianBlur stdDeviation="6" result="blur" />
      <feComposite in="SourceGraphic" in2="blur" operator="over" />
    </filter>
  </defs>

  <!-- 1. Luxury White Squircle Container -->
  <rect x="24" y="24" width="976" height="976" rx="220" fill="#ffffff" stroke="#e2e8f0" stroke-width="6" />

  <g clip-path="url(#squircle-clip)">
    <!-- Centered Coordinate Space (0,0 at center) -->
    <g transform="translate(512, 512)" filter="url(#subtle-shadow)">

      <!-- 2. Massive Hexagonal Architectural Gateway Frame -->
      <polygon points="
        0,-395
        342,-198
        342,198
        0,395
        -342,198
        -342,-198
      " fill="none" stroke="#0f172a" stroke-width="38" stroke-linejoin="round" />

      <!-- Inner Dashed Telemetry Track (Queue Stream Loop) -->
      <polygon points="
        0,-360
        312,-180
        312,180
        0,360
        -312,180
        -312,-180
      " fill="none" stroke="#00f5ff" stroke-width="4" opacity="0.45" stroke-dasharray="16, 12" />

      <!-- Concentric Queue Radar Guides -->
      <circle cx="0" cy="0" r="285" fill="none" stroke="#38bdf8" stroke-width="1.5" opacity="0.14" stroke-dasharray="6, 8" />
      <circle cx="0" cy="0" r="185" fill="none" stroke="#00f5ff" stroke-width="1.5" opacity="0.16" stroke-dasharray="4, 6" />

      <!-- 3. Symmetrical Origami Observability Owl Mascot -->
      <!-- 100% Watertight Solid Base Silhouette -->
      <path d="
        M 0 -220
        L 80 -210 L 155 -285 L 180 -220 L 220 -150 L 265 -30 L 240 70 L 270 160 L 230 250 L 140 295 L 0 325
        L -140 295 L -230 250 L -270 160 L -240 70 L -265 -30 L -220 -150 L -180 -220 L -155 -285 L -80 -210
        Z
      " fill="#060911" />

      <!-- EAR TUFTS & FOREHEAD MESH -->
      <!-- Left Ear (viewer's left: x < 0) -->
      <polygon points="-80,-210 -155,-285 -130,-190" fill="url(#f-highlight)" />
      <polygon points="-155,-285 -180,-220 -130,-190" fill="url(#f-mid)" />
      <polygon points="-180,-220 -220,-150 -130,-140" fill="url(#f-dark)" />
      <polygon points="-130,-190 -180,-220 -130,-140" fill="url(#f-mid)" />

      <!-- Right Ear (viewer's right: x > 0) -->
      <polygon points="80,-210 155,-285 130,-190" fill="url(#f-highlight)" />
      <polygon points="155,-285 180,-220 130,-190" fill="url(#f-mid)" />
      <polygon points="180,-220 220,-150 130,-140" fill="url(#f-dark)" />
      <polygon points="130,-190 180,-220 130,-140" fill="url(#f-mid)" />

      <!-- Crown Center Brow -->
      <polygon points="0,-220 -80,-210 -45,-155 0,-165" fill="url(#f-light)" />
      <polygon points="0,-220 80,-210 45,-155 0,-165" fill="url(#f-light)" />
      <polygon points="-80,-210 -130,-190 -130,-140 -45,-155" fill="url(#f-mid)" />
      <polygon points="80,-210 130,-190 130,-140 45,-155" fill="url(#f-mid)" />

      <!-- Nasal Bridge & Sensor Ridge -->
      <polygon points="0,-165 -45,-155 0,-70" fill="url(#f-highlight)" />
      <polygon points="0,-165 45,-155 0,-70" fill="url(#f-light)" />

      <!-- EYE ORBITS / FACIAL DISCS -->
      <!-- Left Eye Socket Facets -->
      <polygon points="-45,-155 -130,-140 -165,-75 -95,-70" fill="url(#f-dark)" />
      <polygon points="-130,-140 -220,-150 -265,-30 -195,-35" fill="url(#f-obsidian)" />
      <polygon points="-165,-75 -265,-30 -195,-35" fill="url(#f-dark)" />
      <polygon points="-195,-35 -240,70 -160,50 -95,-15" fill="url(#f-mid)" />
      <polygon points="-165,-75 -195,-35 -95,-15" fill="url(#f-dark)" />

      <!-- Right Eye Socket Facets -->
      <polygon points="45,-155 130,-140 165,-75 95,-70" fill="url(#f-dark)" />
      <polygon points="130,-140 220,-150 265,-30 195,-35" fill="url(#f-obsidian)" />
      <polygon points="165,-75 265,-30 195,-35" fill="url(#f-dark)" />
      <polygon points="195,-35 240,70 160,50 95,-15" fill="url(#f-mid)" />
      <polygon points="165,-75 195,-35 95,-15" fill="url(#f-dark)" />

      <!-- BINOCULAR OPTICAL LENSES (The "Lens" in QueueLens) -->
      <!-- Left Lens Assembly -->
      <circle cx="-100" cy="-65" r="54" fill="#0b0f19" stroke="#1e293b" stroke-width="4" />
      <circle cx="-100" cy="-65" r="48" fill="none" stroke="url(#cyan-glow)" stroke-width="5" filter="url(#optic-glow)" />
      <circle cx="-100" cy="-65" r="38" fill="none" stroke="#00f5ff" stroke-width="2" opacity="0.6" stroke-dasharray="10, 6" />
      <circle cx="-100" cy="-65" r="26" fill="url(#cyan-iris)" />
      <circle cx="-100" cy="-65" r="14" fill="#060911" />
      <circle cx="-92" cy="-73" r="5" fill="#ffffff" opacity="0.9" />

      <!-- Right Lens Assembly -->
      <circle cx="100" cy="-65" r="54" fill="#0b0f19" stroke="#1e293b" stroke-width="4" />
      <circle cx="100" cy="-65" r="48" fill="none" stroke="url(#cyan-glow)" stroke-width="5" filter="url(#optic-glow)" />
      <circle cx="100" cy="-65" r="38" fill="none" stroke="#00f5ff" stroke-width="2" opacity="0.6" stroke-dasharray="10, 6" />
      <circle cx="100" cy="-65" r="26" fill="url(#cyan-iris)" />
      <circle cx="100" cy="-65" r="14" fill="#060911" />
      <circle cx="108" cy="-73" r="5" fill="#ffffff" opacity="0.9" />

      <!-- AMBER SENSOR BEAK (Keen Alert Apex) -->
      <polygon points="0,-70 -24,-35 0,25" fill="url(#amber-accent)" />
      <polygon points="0,-70 24,-35 0,25" fill="#ea580c" />
      <polygon points="-24,-35 0,25 0,-15" fill="#f59e0b" />
      <polygon points="24,-35 0,25 0,-15" fill="#d97706" />

      <!-- CHEST & BODY ORIGAMI ARMOR PLATES -->
      <polygon points="0,25 -95,-15 -55,95 0,80" fill="url(#f-light)" />
      <polygon points="0,25 95,-15 55,95 0,80" fill="url(#f-mid)" />

      <!-- Lateral Upper Chest -->
      <polygon points="-95,-15 -160,50 -125,125 -55,95" fill="url(#f-mid)" />
      <polygon points="95,-15 160,50 125,125 55,95" fill="url(#f-dark)" />

      <!-- Mid-Body Armor Chevrons (Queue State Transitions) -->
      <polygon points="0,80 -55,95 0,165" fill="url(#f-highlight)" />
      <polygon points="0,80 55,95 0,165" fill="url(#f-light)" />

      <polygon points="-55,95 -125,125 -70,195 0,165" fill="url(#f-dark)" />
      <polygon points="55,95 125,125 70,195 0,165" fill="url(#f-obsidian)" />

      <!-- Lower Wing Flanks & Claws/Plates -->
      <polygon points="-160,50 -240,70 -270,160 -195,180 -125,125" fill="url(#f-obsidian)" />
      <polygon points="160,50 240,70 270,160 195,180 125,125" fill="url(#f-obsidian)" />

      <polygon points="-270,160 -230,250 -160,240 -195,180" fill="url(#f-mid)" />
      <polygon points="270,160 230,250 160,240 195,180" fill="url(#f-dark)" />

      <polygon points="-195,180 -160,240 -70,195" fill="url(#f-mid)" />
      <polygon points="195,180 160,240 70,195" fill="url(#f-dark)" />

      <!-- Bottom Apex Shield (Tail/Keel) -->
      <polygon points="0,165 -70,195 -45,260 0,250" fill="url(#f-light)" />
      <polygon points="0,165 70,195 45,260 0,250" fill="url(#f-mid)" />

      <polygon points="-70,195 -160,240 -140,295 -45,260" fill="url(#f-dark)" />
      <polygon points="70,195 160,240 140,295 45,260" fill="url(#f-obsidian)" />

      <polygon points="0,250 -45,260 0,325" fill="url(#f-mid)" />
      <polygon points="0,250 45,260 0,325" fill="url(#f-dark)" />
      <polygon points="-45,260 -140,295 0,325" fill="url(#f-obsidian)" />
      <polygon points="45,260 140,295 0,325" fill="url(#f-obsidian)" />

      <!-- Electric Cyan Core Heartbeat Pip -->
      <circle cx="0" cy="115" r="5" fill="#00f5ff" filter="url(#optic-glow)" />
      <circle cx="0" cy="115" r="2.5" fill="#ffffff" />
    </g>
  </g>
</svg>`;
}

async function renderLogo() {
  const svg = buildLogoSvg();
  const outputDir = path.resolve(__dirname, '../docs/images');
  if (!fs.existsSync(outputDir)) {
    fs.mkdirSync(outputDir, { recursive: true });
  }

  const svgPath = path.join(outputDir, 'logo.svg');
  const pngPath = path.join(outputDir, 'logo.png');

  fs.writeFileSync(svgPath, svg, 'utf8');

  const resvg = new Resvg(svg, {
    fitTo: { mode: 'width', value: 1024 },
  });
  const pngData = resvg.render().asPng();
  fs.writeFileSync(pngPath, pngData);

  console.log('✓ Successfully rendered logo.svg and logo.png to docs/images/');
}

renderLogo().catch((err) => {
  console.error('Failed to generate mascot logo:', err);
  process.exit(1);
});
