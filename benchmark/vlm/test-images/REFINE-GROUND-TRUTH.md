# Refinement ground truth — 7 new UI/OCR/code images

**STATUS:** `extract-code-test-1`, `ocr-test-1`, `ocr-test-2` — owner-verified ✓. `ocr-test-3`, `ocr-test-4`, `ui-test-1`, `ui-test-2` — VLM-derived reference. For the two long text images full review is impractical, so once all model outputs are in we **diff every variant vs this reference and surface only the disagreements (especially handwritten fields) for owner adjudication**, then finalize GT and score. Benchmark rows: `run_id=refine-*`.

---

## extract-code-test-1.png — Google Apps Script (JavaScript)  *(owner-verified 2026-06-23)*
A playlist-builder function. Language: **JavaScript (Google Apps Script)**.
```javascript
function createAujourdhui() {

  var C = PLAYLIST_CONFIG;
  validatePlaylistKeys_(['AUJOURDHUI', 'ALL_ENGLISH', 'ALL_LATIN', 'REGGAE_ALL', 'CHANSONS_FRANCAISES', 'ROMANCE', 'AUJOURDHUI_SOURCE_6']);

  let name = 'La playlist d\'aujourd\'hui';
  let id = C.AUJOURDHUI.id;
  let historyFile = 'aujourdhui.json';
  let playlistSize = 200;
  let historyDays = 6;
  let today = (new Date()).toISOString().split('T')[0];
  let description = "[DAILY UPDATE " + today + "]";

  Logger.log("Creating custom playlist: " + name);

  try {
  // Load previously saved tracks
  let tracksPrevious = Cache.read(historyFile);
  Logger.log(Math.round(tracksPrevious.length) + ' tracks from recent playlist loaded');

  // Get all tracks from source playlists (lightweight: only id/name/uri/duration_ms/artists)
  let tracks = [];
  rateLimitedForEach([C.ALL_ENGLISH.id, C.ALL_LATIN.id, C.REGGAE_ALL.id,
    C.CHANSONS_FRANCAISES.id, C.ROMANCE.id, C.AUJOURDHUI_SOURCE_6.id
  ], id => Combiner.push(tracks, getPlaylistTracksLightweight(id)), 500);
  Logger.log(Math.round(tracks.length) + ' total tracks available from source playlists');
  // Remove duplicates
  Filter.dedupTracks(tracks);
  Logger.log(Math.round(tracks.length) + ' tracks left after removing duplicates');
  // Remove the tracks loaded from the previous history
  Filter.removeTracks(tracks, tracksPrevious);
  Logger.log(Math.round(tracks.length) + ' tracks left after removing selections from the past ' + historyDays + ' days');
  // Only keep a specified number of tracks, randomly selected
  Selector.keepRandom(tracks, playlistSize);
  Logger.log(Math.round(tracks.length) + ' tracks randomly selected for the new playlist');

  // Save playlist, without modifying the cover
  Playlist.saveWithReplace({
    id: id,
    name: name,
    tracks: tracks,
    description: description,
    public: true
  });

  // Add new playlist tracks to history
  // Cleanup the tracklist by removing unnecessary information
  Cache.compressTracks(tracks);
```
**Hard discriminators** (easy to get wrong — what separates good from bad OCR here): `createAujourdhui` (no apostrophe), `validatePlaylistKeys_` (trailing underscore), `Combiner.push` (not "Combine"), `'La playlist d\'aujourd\'hui'` (escaped apostrophes inside the string). **The screenshot is bottom-truncated at `Cache.compressTracks(tracks);`** — there is no try/catch closing visible, so any appended `catch` block / closing braces / trailing code is a fabrication (not a successful read).

## ocr-test-1.jpg — Spanish anti-bullfighting poem (handwritten)  *(owner-verified 2026-06-23)*
```
TENDIDO ZERO SENTIDO
TA-TA-TA → TROMPETA INICIO CORRIDAS ✖
HAS OIDO LAS TROMPETAS?
YA EMPEZO LA EXPOSICION
SON ARTISTAS CONCLAMADOS
CON COLETA Y SUS MULETAS
UNA ESPADA ENTRE SUS MANOS
Y EL PAQUETE BIEN MARCADO ✖
VISTEN UN TRAJE DE ✖LUZ✖ (LUCES)
Y A SU VIRGEN HAN REZADO
BIEN SOLITOS DE RODILLAS
VIGILADOS POR CUADRILLAS
UN DE OTRO ENAMORADOS ✖
PREPARADOS Y ENTRENADOS
ARRIESGARAN SUS TRISTES VIDAS
POR UNOS HIJOS DE PUTAS
LLAMAN ARTE, TAUROMAGIA
UN VERDADERO MATADERO ✖
MEGAEVENTO HASTA INSULTANTE
TODA FORMA INTELIGENTE
YO QUE SOY UN IGNORANTE
NO TARDE EN DARME CUENTA
QUE MAS QUE ARTE ES MATANZA ✖
NADA HERMOSO O FASCINANTE
VEO EN HACER CORRER LA SANGRE
DE TAN NOBLE, BRAVO Y MUSCOLADO
PA' MI FANTASTICO ANIMAL
```

## ocr-test-2.jpg — handwritten fitness routine  *(owner-verified 2026-06-23)*
```
Kick thru
Downdog (exhale)
Bear pose (inhale)
Kick-thru (exhale)
Bear pose (inhale)

Func getup
Inverted table-top
Rotate then get-up
Squat then reset

3 point row
Flat back, one hand on chair
Row front of body to waist8x side 1 + 8x side 2

Side planks with rotation
8x side 1
8x side 2

Push-ups
with hands release

Medecine ball twist

Abs
Mountain climbers
or alternating knees
```

## ocr-test-3.jpg — German product label (SUBSTRAL Ameisen-Köder)
Key verbatim strings: `SUBSTRAL`, `Naturen`, `Ameisen-Köder plus Nestwirkung`, `Spinosad`, `0,89 g/kg`, `AT-0030275-00000`, `Evergreen Garden Care Österreich GmbH`, `Franz-Brotzner-Straße 11-13, A-5071 Wals-Siezenheim`, `Tel: +43 (0)662/453713 300`, `www.substral.at`, `PT 18`, `Lasius niger`, `10 g`, `2 Jahre`, `35°C`, `GIFTINFORMATIONSZENTRUM`, `+43 (0)1 406 43 43`, `53103g`, `1,2-Benzisothiazol-5(2H)-one`. (Long instructions text — full reference available in extraction; score on the key fields above + overall fidelity.)

## ocr-test-4.png — Spanish property-registry form (printed + handwritten)
Header *(verified 2026-06-26)*: `Registro de la Propiedad de Corralejo, T.M. La Oliva. Isla de Fuerteventura.`, `Doña María Isabel Cabra Rojo, Registradora de la Propiedad.`, `Tel: 928 537 272 Fax: 928 866 469`, `C/ Bajo Blanco, 2, Edificio Hubara, 35660`, website `www.registradores.org`, registry email `corralejo@registrodelapropriedad.org`. (Note: registradores.org is the website; the email domain is registrodelapropriedad.org — two different domains both printed.)
Form title: `SOLICITUD DE NOTA SIMPLE INFORMATIVA`, `Nº Borrador: 42 / 2025`, `Nº Entrada: 11656 / 2025`.
Handwritten applicant: `Joan`, **`Almiñana Moropsa`** *(owner-verified 2026-06-23)*, `D.N.I.: 20041185C`, `Teléfono/s: 635889002`, **`C/ Gerardo estevet Martin n°19 piso 3A`** *(owner-verified)*, `joansnobc@gmail.com`.
Property table fincas: `32840`, `32885`, `32803`, `32871`. Titular: `Adelina Muñoz Archanas 50183525D`. (Plus GDPR/legal paragraphs.)

## ui-test-1.png — "Jan" desktop app, Settings page  *(re-verified 2026-06-26)*
App: **Jan** (v0.8.2). **Three columns**: left nav sidebar + middle settings-category column (e.g. Appearance/Assistants-style entries) + right settings content. Top-right corner has the macOS traffic-light dots **plus extra icons** (download/grid/split-view). Provider-list entries carry **colored per-provider icons**.
- **Left nav**: window-control dots; items `New Chat` `⌘N`, `New Projects` `⌘P`, `Search` `⌘K`, `Hub`, `Settings` (active) — **keyboard shortcuts ARE shown** next to these items. Sections `Chats` (recent: "Build a PacMan Clone…", "Translate the following…", "interpreter"), `INTEGRATIONS` (`Experimental` tag, `MCP Servers`, `Claude Code`), `MODEL PROVIDERS` (`+` add; `LOCAL`: `Llama.cpp`, `MLX`; `REMOTE`: `OpenAI`, `Azure`, `Anthropic`, `OpenRouter`, `Mistral`, `Groq` (last fully-visible entry), `x.AI` (last entry, **truncated** — only the top ~40% visible; models often render it "xAI")).
- **Right (Settings)**: sections `General` (`App Version` `v0.8.2`; `Automatic Update Check` toggle **on** + button `Check for Updates` **enabled**; `Language` dropdown `English`), `Data Folder` (`App Data` path `/Users/frederic/Library/Application Support/Jan/data` + `Change Location`; `App Logs` + `Show in Finder` / `Open Logs`), `Advanced` (`Jan CLI` at `/Users/frederic/.local/bin/jan` + `Uninstall`; `Reset To Factory Settings` + red `Reset` button).

## ui-test-2.png — AI image-generation app (Default Project, failed queue)
Three panes: left project sidebar, center History/Queue, right Configuration.
- **Left**: `Default Project`, `Projects` + `+`, `Active` / `Default Project` (selected), `Estimated: $0.00`, `Settings`.
- **Center**: tabs `Staging`/`Results`/`History`(active); `Default Project History`, `20 edits found`; history items with prompts ("A hand-drawn felt marker modern art professional i…"), models (`gemini-3-pro-…`, `gpt-image-2`), `4K`, `1:1`, `$0.00`, **`FAILED`** (red) + errors (`Unknown error occ…`, `API error (400): (…`). `Queue` / `Show Logs`; `Issues (1)` / `Clear All`; `Gemini_Generated_Image_eoyylleoyylleoyy.png`, `Unknown error occurred.`, `Queue finished with issues`, `Hide Queue`.
- **Right (Configuration)**: tabs `Image`/`Text`; `Generate Image` (faded blue, greyed text = **disabled**); `VARIATIONS`; `OUTPUT LOCATION` `Downloads` `/Users/frederic/Downloads`; `PROMPT` "Write or select a prompt…"; `ASPECT RATIO` `Auto Mode`; `SIZE` `4K`; `BATCH TIER` "50% cost savings." (toggle off); `MULTI-INPUT MODE` "Merge all to 1 output." (off); `PROJECTED COST` `≈ $0.00`; inputs/outputs/total table (`0 images @ 4K`, `$0.0011 each`, `$0.240 each`, `Standard tier`, `gemini-3-pro-image-preview`).
