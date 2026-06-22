# Ground Truth: Test Image Descriptions

Verified by the image owner (Frederic Guigand). Used for automated scoring in `score_v3.py`.

---

## 01_ui_login.png — macOS login mockup

Mockup (not a real screenshot) of a macOS login form. Three round buttons (red, orange, green) in the top left corner (macOS traffic lights). Header reads **"Sign in to Acme"**. Email field with placeholder `user@example.com`. Password field (masked with dots). Unchecked square checkbox below the entry fields, just before the text **"Keep me signed in"**. Red box with red text on pale red background: **"Invalid credentials. Please try again."** Blue **"Sign In"** button with white text. At the bottom of the form, left-aligned small blue text: **"Forgot your password?"**.

---

## 02_code_python.png — Python FastAPI snippet

Screenshot of Python code in a dark-themed editor. Code defines a `User` Pydantic model and a FastAPI POST endpoint `/users` that creates a user and commits to a database. ~15 lines of code including imports, class definition, decorator, and async function.

---

## 03_error_trace.png — Python stack trace

Terminal screenshot. Command `$ python app.py` triggers a chain of errors:
- **psycopg2.OperationalError**: connection to server at `"db.internal"` (10.0.0.5), port 5432 failed: FATAL: password authentication failed for user `"app"`
- During handling, a secondary **ConnectionError** occurs: "Failed to connect: password authentication failed for user 'app'"
- Stack trace references `/Users/alice/app/main.py` line 42 (`db = connect(DB_URL)`) and `/Users/alice/app/db.py` line 18 (`raise ConnectionError(...)`)
- Process exits with code 1

---

## 04_architecture.png — System architecture diagram

Technical diagram titled **"System Architecture"**. Left-to-right flow:
- **Web Client**: light blue rounded rectangle
- **API Gateway (FastAPI)**: yellow/orange rounded rectangle
- **Auth Service**: green rounded rectangle (top branch)
- **Worker Queue**: green rounded rectangle (bottom branch)
- **Postgres**: pink oval (connected to Auth Service)
- **Redis**: pink oval (connected to Worker Queue)
- Protocol labels: **HTTPS** (Web Client → API Gateway), **gRPC** (API Gateway → Auth Service / Worker Queue)
- All rectangles have rounded corners; databases are ovals
- **Notable design quirk**: the gRPC line from API Gateway splits into 3 paths — 2 go to Auth Service and Worker Queue, but a third line in the middle goes nowhere (dangling/phantom connection)

---

## 05-cropped-youtube-capture.png — Shaolin monk practicing Qigong

Cropped YouTube screenshot. A **Shaolin monk** practicing **Qigong Yi Jin Jing** in what appears to be a traditional Chinese temple or shrine setting. The monk wears traditional robes. Indoor setting with architectural elements (columns, pillars, walls).

**Chinese text elements (verified by image owner 2026-06-19)**:
- **Vertical wall scroll** (behind monk): **茶香四溢满屋** — "the fragrance of tea fills the room"
- **Red altar plaque** (right side): **功德** — merit / virtue
- **Additional altar text**: **佛** (Buddha) and **广种福** (widely plant blessings — note: 3 characters, not the 4-character idiom 广种福田)
- **Bodhidharma scroll painting** with additional vertical text on the side — too small to read accurately
- **Chinese knot decoration** hanging from ceiling (NOT text-bearing)

Transcribing the listed characters is CORRECT behavior. Minor OCR errors on individual characters (e.g. 恭 vs 茶, 田 vs 四) are minor errors, not hallucinations. Inventing characters in places where no text exists IS hallucination.

**Framed photograph on the wall** (behind the monk's head, verified 2026-06-19): A framed photo showing **the Pope (probably Francis)** on the left, facing **another non-identifiable person** (probably a Shaolin prominent figure). Models identifying the Pope are CORRECT, not hallucinating.

**Flag (verified by image owner 2026-06-19)**: The flag visible on the left side of the image is the **Austrian flag** (red-white-red horizontal stripes). Common model errors:
- "Canadian flag" — WRONG (different design, has maple leaf)
- "Japanese flag" — WRONG (white with red circle)
- "Indonesian flag" — WRONG (red-over-white horizontal halves)
- "Chinese flag" — WRONG (red with yellow stars)
- Generic "red and white flag" without country identification — PARTIALLY CORRECT (gets the colors right but lacks specificity)

---

## 06-trading-card.jpg — Pacman yin-yang trading card

Virtual digital **trading card** with a **pacman theme**. Features a **yin and yang symbol** rendered in **pop art style**. Stylized, vibrant, bold colors.

**Yin-yang orientation** (verified by image owner 2026-06-19):
- **LEFT side**: YELLOW, with a **RED GHOST** in the dot position
  - Ghost eyes: WHITE, looking to the RIGHT
- **RIGHT side**: RED, with a **YELLOW PACMAN** in the dot position
  - Pacman mouth: open to the RIGHT
- Black broken circular lines within the yin-yang reflect the circle shape and the black circle outline from the background

**Background**: Plain **CYAN** with **concentric BLACK CIRCLES** (not spirals, not radial). The black circles echo/reflect the round shape of the yin-yang.

**Common model errors** (these are real visual mistakes, not acceptable paraphrasing):
- Saying Pacman is on the left (it's on the RIGHT)
- Saying Ghost is on the right (it's on the LEFT)
- Saying background is "spiral" pattern (it's concentric circles)
- Saying background color is anything other than cyan
- Saying the halves are white/black (they're yellow/red)

---

## 07-photo-massage-therapists.jpg — Massage therapists promotional photo

**5 massage therapists** (**2 men, 3 women**) posing for a **promotional photo** in a massage room.

**Massage table (center foreground)**:
- 2 rolled white towels on top
- Blue runner underneath the towels
- White sheet underneath the blue runner
- A rolled and folded white towel positioned **in front of the therapists** (further right) for head support

**Plants**:
- **Two bamboo plants** in the background — one in the **left corner**, one in the **right corner**
- A **succulent plant in a pot** on the **left side, left of the left bamboo**

**Furniture**:
- **Low grey/white cupboard** on the **left**
- **Low grey/white cupboard** on the **right**
- A **tall grey/white cupboard** closer to the viewer (also on right)
- A **blue folded towel** is visible on the right low cupboard (matches the blue runner)

**Background**:
- **4 windows** with **blinds pulled down but opened**
- Another building faintly visible through the windows
- Bright natural lighting

**Other elements** (verified 2026-06-19):
- A **partly visible office chair** behind the therapists (4th-person area) — models mentioning this are CORRECT
- Floor: **light brown cork** (acceptable to call it "wooden" — easy to mistake)

---

## 08-poster-class-schedule.jpg — VIC Health Club weekly schedule

**Weekly Activities Schedule** for **VIC Health Club**. Each class is displayed in a colour-coded rounded **pill** shape containing: a black **icon on a white disc** (left side), and white text on multiple lines with times, class title, and optional additional info.

**Colour groups (verified 2026-06-19)**:
- Blue = Yoga (5 classes)
- Orange = Shaolin (3 classes)
- Green = Fitness (3 classes)

**Full schedule (verified 2026-06-19)**:

| Day | Time | Class | Notes |
|---|---|---|---|
| Monday | 12:00-13:00 | **Shaolin QIGONG** | orange/Shaolin |
| Monday | 13:00-14:00 | **YOGA** | with the Embassy of India |
| Monday | 13:10-14:00 | **OSTEOPOROSIS Prevention** | (overlapping slot) |
| Tuesday | 12:00-13:00 | **YOGASTHENICS** | green/Fitness |
| Tuesday | 13:00-14:00 | **YOGA** | with the Embassy of India |
| Tuesday | 13:00-14:00 | **TAIJI (24 forms)** | with Gabi |
| Wednesday | 12:00-13:00 | **CALISTHENICS** | w/ VIC Runners |
| Wednesday | 13:00-14:00 | **YOGA** | |
| Thursday | 12:00-13:00 | **Shaolin KUNG FU** | orange/Shaolin |
| Thursday | 13:00-14:00 | **YOGA** | |
| Friday | 13:00-14:00 | **PRANAYAMA** | |

Total: 11 classes across Mon-Fri, all in lunch hour slots (12:00-13:00 or 13:00-14:00).

**Icons per type**:
- Same silhouette doing martial arts → all Shaolin practices (Qigong, Kung Fu)
- Same sitting silhouette meditating or yoga posing → all Yoga classes
- Head blowing wind → Pranayama
- Gym weight → Yogasthenics
- Person running → Calisthenics
- Person standing up from a squat → Osteoporosis Prevention

Footer: black bar with **"Register online at https://vichealth.club"** and a QR code. Background has faint lotus flower and ink-wash brush stroke decorations.

---

## 09-poster-artnight.jpg — Artnight event poster

**Digital poster** whose entire area is a **photograph of a painting** (medium is probably acrylic, but could be oil — models that say either should not be heavily penalized).

**Painting subject**: A **red/orange fiery horse** dominates the foreground, over a much **darker swirly background in shades of blue** with subtle yellow, brown, green, and orange accents. **Visible brush strokes and intentional paint thickness (impasto)**.

**Trees**: On the right side, in front of the horse, are **3 stylized trees with brown/orange leaves**. The trees appear to be **suspended in the air** (deliberate artistic choice, not a naturalistic scene).

**Top-left corner** (2 white logos side-by-side):
- **VIC Health Club**: laurel branches surrounding hands supporting a heart (same design as image 17 standalone logo)
- **VIC Art Club**: laurel branches with a statue of a woman next to an inverted statue

**Title** (3 lines, positioned below the horse):
- Line 1: **"PAINT YOUR MASTERPIECE"** — medium-thin dark-orange sans-serif font
- Line 2: **"ARTNIGHT"** — large white serif title font (the main event name)
- Line 3: **"TUESDAY 14 APRIL | 17:00 | THE VIC"** — medium white sans-serif font

**Bottom-right corner**: A **QR code** with caption on 2 lines:
- "Scan to register"
- "vichealth.club"

---

## 10-photo-rice-porridge-breakfast.png — South Asian breakfast

**South Asian breakfast** (e.g., Singapore style). **Rice porridge** (congee) topped with:
- **Sunny side up egg** with **runny yolk** (unusual for Asian eggs)
- Clockwise from 12:00: chopped **parsley or coriander**, crispy **shallots or onions**, **Chinese soup spoon** handle sticking out, sprinkle of **soy sauce**, chopped **spring onions**, maybe thin **ginger sticks**

All in a **white ceramic/porcelain/china bowl**. 

Additional items on the table:
- **Right of the bowl**: hot fresh **soy milk** (tahue milk) on a **wooden plate**
- **Left of the bowl**: chopped **you tiao** (aka you char kway / Chinese fried dough) on a **red plate**
- **Brown chopsticks** appear to rest on the red plate

All on a **marble table**. The photo appears to be **AI-generated** due to some inconsistencies like the way the chopsticks are resting, which seems unnatural.

---

## 11-collage-therapist-photo.jpg — Massage therapist diptych

Two side-by-side photographs of the **same massage therapist** (same person in both):
- **Left photo**: posing on a **massage chair**
- **Right photo**: sitting on a **massage table** with accessories for **Thai bamboo stick massages**

---

## 12-manga-nausicaa-colour.png — Nausicaä manga page

Manga page from **Nausicaä of the Valley of the Wind** (Hayao Miyazaki), **coloured edition**, **page 10**. The scene depicts when **Nausicaa discovers the Ohm shell**. Dark, industrial or cavernous setting. **11 panels** total.

**Key event in frame 9 (verified 2026-06-19)**: **Nausicaa detonates gunpowder** to detach the **transparent eye dome** from the **Ohm shell**. The explosion IS present in the image — models describing an explosion in frame 9 are correctly reporting the scene, NOT hallucinating.

---

## 13-colour-swatch-nausicaa.jpg — Nausicaä color swatch

Minimalist color palette / swatch design inspired by **Nausicaä of the Valley of the Wind**. **10 colours** displayed vertically. **No hex codes** printed on the image. Text in the bottom right corner reads **"The colors of"**.

---

## 13-marker-drawing-spotify-cassette.jpeg — Marker drawing with French text

**Wide tip markers drawing** depicting a **cassette tape** (Spotify-themed). Contains handwritten text in **French**: **"musique à volonté"** (meaning "music at will / on demand").

---

## 14-screenshot-onerpm-catalog.jpg — OneRPM music catalog

Screenshot of the **OneRPM** music distribution platform's **"My Catalog"** dashboard. User: **Frederic**. Account: **Neurological Waves Science**. URL: `dashboard.onerpm.com`.

**4 artists:**
- Neurological Waves Science
- La Sonora Volcánica
- Amadi
- Frédéric Guigand

**15 albums with visible titles:**
Winding Down, Silk and Steel, Waikiki, TENDIDO CERO SENTIDO, A Sky Remembered, Cumbia del Barrio, Presque Louis, Gaelic Cradle Song, Endless Ocean, Numb, Where the Sea Remembers, Spring's First Light, Flight of the Phoenix, London, Something's Wrong

**5 albums where the title is NOT visible in the listing but can be read (difficult) from the album cover thumbnail:**
Secreto del Sur, Sol Sol, Ocean Flute, Spirits of the Wind, Timeless Whispers

**Status tags:**
- 17 albums = **Distributed** (green)
- 1 album = **Incomplete** (Something's Wrong — red exclamation mark)
- 2 albums = **Approved** (Secreto del Sur, SOL SOL — green)

**Presque Louis** by Frédéric Guigand is highlighted with a **yellow rectangle** (selected/focused).

---

## 16-album-cover-lofi.jpg — Lo-fi album cover

Square-format album/playlist cover with **lo-fi / vaporwave aesthetic**. Interior space looking out through windows at night. The text **"songs to stare at the ceiling to"** appears **ONCE** (at the bottom of the image). Heavy film grain, intentional light blurs, dreamy atmosphere.

---

## 17-logo-vic-health-club.png — VIC Health Club logo

Minimalist, **monochrome** (black and white) logo. Three symbolic elements in solid black:
- A **heart** at the center top
- Two stylized **hands** (palm-up, open) flanking the heart, appearing to cradle or support it
- A **laurel wreath** (circular arrangement of leafy branches) encircling the heart and hands

Text below: **"VIC Health Club"** in bold black sans-serif font. Plain white background.

---

## 18-qrcode-vic-health-club.png — QR code

**Plain black-and-white QR code** on a white background. **NO globe icon, NO symbol, NO central decoration** — just a standard QR code pattern (verified by image owner 2026-06-19). Models claiming to see a "globe in the center" or any other central icon are hallucinating.

When decoded, contains the URL: **https://vichealth.club** (not visible in the image itself — only inferable from the schedule-poster context).

---

## 19-watercolour-painting-surfers-fuerteventura.png — Watercolor painting

**Watercolour painting** using a **luminous neon pastel colour palette**. Depicts the **north coast of Fuerteventura**.

**Setting:** A large **purple mountain** is visible in the background across the sea — this is **Lanzarote** visible across the strait, which places the location on the north coast of Fuerteventura.

**Two surfers (verified by image owner 2026-06-19):**

1. **First surfer (foreground, left):** A **man** with **short BROWN hair**, seen from the **back**, **sitting** on **black volcanic rocks** on the left side of the painting. He is wearing a **shortie wetsuit** that is **between BLUE and PURPLE** in color. He is **watching** the second surfer.

2. **Second surfer (center):** A **male** surfer, **surfing a medium-to-large wave** (slightly overhead, ~2.5m high). He has **just taken off** and is going **straight** — he has not made a turn yet. He is right below the peak. The wave is a classic **A-frame**: just beginning to break at the peak, but otherwise clean. The portion nearer the viewer/coast is close to the rocks and already breaking. The portion behind the surfer is clean and looks like it will develop into a nice **long left-hand turn** (this is where he should turn). The surfer is **facing the viewer** with his board (**short board**) pointing to the **right** of the image, which means he is **goofy footed**.

**Common model errors to verify**:
- Wrong surfer count (anything other than 2)
- Describing only one surfer (missing the foreground observer)
- Wrong hair color for foreground observer (it's BROWN)
- Wrong wetsuit color (it's BLUE-PURPLE, not generic "dark")
- Wrong board direction (it points to the RIGHT)

---

## 20-ultrawide-kung-fu-banner-with-logo.png — Shaolin kung fu banner

Wide panoramic banner featuring **9 people** performing **Shaolin kung fu** poses against a plain wall with a wooden floor.

**Left badge (verified 2026-06-19):**
- **Round** badge with **YELLOW border**
- **YELLOW text and line drawing** (not red/maroon background, not gold-on-red)
- Text reads: **"SHAOLIN TEMPLE ÖSTERREICH"** (Shaolin Temple Austria)
- **3 Chinese symbols on top** (likely 少林寺 = Shaolin Temple, but possibly other characters — image owner cannot fully verify)
- **Center of badge: yellow line drawing of BODHIDHARMA** (the legendary founder of Zen/Chan Buddhism)

**Right logo:** **BLACK** emblem depicting **two hands supporting a heart**, surrounded by **laurel leaves/wreath** (same design as standalone VIC Health Club logo in image 17).

**Performers (verified):**
- **2 Chinese Shaolin monks** wearing **darker robes**, positioned in the **CENTER** — they have **NO WEAPONS**
- **7 Shaolin disciples** wearing **LIGHT BLUE clothes** on the sides

**Weapons distribution (verified):**
- 3 disciples have **1 sword each**
- 1 disciple has **2 swords**
- 1 disciple has a **lance**
- 1 disciple has a **shield**
- 1 disciple has a **staff**
- Total: 7 weapons across 7 disciples (monks unarmed)
- Red cloth flags attached to several weapons

**Common model errors**:
- Wrong people count (not 8, not 10)
- Wrong monk count (not 1, not 3+)
- Saying monks have weapons (they don't)
- Saying badge background is red/maroon (it has yellow border + yellow text/drawing)
- Misspelling ÖSTERREICH (umlaut matters)

---

# Images 21-30 (added 2026-06-19) — DRAFT ground truth from z.ai MCP analysis, PENDING owner verification

## 21-motion-blur.jpg — Long-exposure urban night street scene (Hong Kong)

Long-exposure photograph of a busy **urban street at night** in a Chinese-speaking city, **most likely Hong Kong**. Vehicle light trails streak horizontally along the street; pedestrians at the bus stop are blurred from motion.

**Composition**:
- Camera looks down the length of a wide multi-lane road with **tram tracks** and a **yellow pedestrian crossing** in the foreground
- Buildings frame both sides (dense skyscrapers)
- Static central feature: a **white cylindrical bus stop / kiosk** on the sidewalk
- **Bus info board above the bus stop** — white and green, with **green bus and tram symbols inside a white circle** (text not legible)
- **Red pedestrian traffic lights** on both left and right sides of the crossing
- Light trails converge toward a vanishing point in the distance

**Color palette**: Cool blues/whites on the left, warm yellows/oranges/reds on the right (split tone).

**Visible signage (verified 2026-06-19)**:
- **Dah Sing Bank 大新銀行** — appears on **BOTH sides** of the street (red-and-white branding)
- **DVFX currency exchange** — on the left
- Various other neon signs and storefronts
- People sitting inside the bank on the right (visible through window)

**Technique**: Long exposure (several seconds), camera on tripod. Static elements (buildings, street, bus stop structure) sharp; moving vehicle lights and waiting pedestrians blurred.

---

## 22-spritesheet-bubble-bobble.png — Bubble Bobble spritesheet (fan-made)

**Fan-made spritesheet** by **"125scratch"** based on Taito's "Bubble Bobble" (1986 arcade release). Arranged on a black background.

**Title (top)**:
- "BUBBLE BOBBLE" — **red letters on a yellow cloud-shaped background with pink borders** (verified 2026-06-21: GT was correct; models misreading letters as pink/peach are wrong)
- Sprites of main characters **Bub (green dragon, player 1)** and **Bob (blue/purple dragon, player 2)**

**Enemy types depicted** (visible, NOT labeled with text): Beluga, Bonze, Invader, Monsta, Pulpul, Hidegons. **Also: ghosts** (verified 2026-06-21; some are the actual enemies, depicted as ghost-like sprites).

**Item sprites visible** (verified 2026-06-21; many present, including but not limited to):
- Sweets: cake slice, cupcake, ice cream, cherry, strawberry, grapes
- Fruits: watermelon slice, apple, mango, peach, banana, pear
- Vegetables: aubergine, carrot, onion, radish, cucumber, mushroom, corn
- Other foods: egg, chicken, sushi, beer

**Other sprite categories**:
- "HELP!!" speech bubbles
- "Hurry up!!" text sprite
- "Super" text sprite
- Circular bubble icons (some with faces)
- Main character animation states (blowing bubbles, inside bubbles, dying)
- HUD elements: "EXTEND", "NICE 1P!", "NICE 2P!", "10PTS!!", "HAPPY END!", digits 0123456789, **"ROUND"** (not "SOUND"), "CLEAR"
- **Individual letter bubbles** for E-X-T-E-N-D (small and large sizes)
- **Colour palette swatch** (reference colors used in the sprites)

**Credit text (bottom, verbatim)**:
> "Sprites made by 125scratch, please give credit if used. Bubble Bobble belongs to Taito. Credit goes to Pac-Man Red on AtariAge for his Atari 7800 BB sprites, which I used in many cases."

**Style**: 8-bit/16-bit arcade palette, vibrant greens/purples/blues/yellows.

**Note for judging**:
- The **green dragon is the player's character (Bub)**, NOT an enemy/boss
- The **blue/purple dragon is player 2 (Bob)**
- "Super Dragon" is NOT a separate boss; it's a power-up state
- "Pink cumin seed" is a hallucination — no such item exists
- "SOUND" is wrong; the text is "ROUND"
- "Bobbie" is a misreading of "Bob"

---

## 23-animation.jpg — 6-frame cartoon cat RUN cycle

**2x3 grid** of 6 numbered animation frames showing a simple **blue cartoon cat** in a **run cycle** (complete looping animation). Frame numbers 1-6 visible.

**Character design**:
- Bright blue body, pink inner ears, white muzzle, black nose
- Stylized / minimalist flat 2D animation
- Western/cartoon style (not anime)

**Action sequence** (1→6): Run cycle — legs and body move through phases of a stride, ending in a crouch/leap position. The 6 frames form a complete loop.

**Background**: Solid light blue. Simple flat shadow directly under the cat in each frame.

**Composition**: Frames arranged in a 2×3 grid with numbers 1-2-3 top, 4-5-6 bottom.

---

## 24-lymphatic-system-handdrawn.jpg — Hand-drawn lymphatic system diagram

Hand-drawn educational diagram of the **human lymphatic system** on lined notebook paper.

**Title**: "Human Lymphatic System"

**Labeled anatomical structures** (with color coding):
- **Tonsils** (throat) — **pink**
- **Thymus** (upper chest) — **orange**
- **Spleen** (upper left abdomen) — **purple**
- **Lymph Vessels** — **green** lines (network throughout body)
- **Lymph Nodes** — **green** dots along vessels
- **Bone Marrow** — **yellow**, depicted in a single leg bone

**Functions list** (handwritten, under the section title **"Functions of Lymphatic System:"**):
- "Helps fight infections."
- "Carries lymph fluid through vessels."
- "Filters harmful substances in lymph nodes."
- "Supports the body's immune defense."

**Style and physical details**:
- Labeled with directional arrows from each structure to its name
- Simple **body outline drawn in gray pencil** with **multiple overlapping lines** (sketchy, erasure marks visible)
- Two **grey smudges** visible: one next to the right hand, the other next to the right foot
- Pen (blue ink) and gray pencil on lined notebook paper

---

## 25-mri-brain-parkinson.jpeg — Brain MRI (2×3 grid of slices, Parkinson's article)

**Six brain MRI slices** arranged in a 2×3 grid, taken from an article illustrating **Parkinson's disease detection with MRI**. Sequence weighting not verified by owner (not a medical expert) — appears T1-weighted (white matter brighter than gray).

**Views**:
- Top-left: Coronal (frontal view, facial structures visible)
- Top-middle: Sagittal (midline)
- Top-right: Axial (at lateral ventricle level)
- Bottom-left: Coronal (posterior, basal ganglia/thalamus)
- Bottom-middle: Sagittal (brainstem/cerebellum)
- Bottom-right: Coronal (facial sinuses)

**Orientation markers (verified 2026-06-19)**:
- "R" (right) visible on multiple slices
- "MF" visible on some slices
- **"AL"** visible in the middle picture of the bottom row (in addition to "MF")

**Scale**: 5cm scale visible on **all pictures EXCEPT the middle column**.

**Annotations**: NO markers, arrows, or annotations visible on the scan itself.

**Metadata**: No patient ID, date stamps, or institutional metadata visible.

**Note for judging**: The Parkinson's-specific features (substantia nigra changes, iron deposition) are not visually obvious and may not be identifiable from these slices alone. Models should not be penalized heavily for not naming specific Parkinson's markers — describing the brain anatomy accurately is sufficient. Models claiming to see specific lesions/annotations that aren't there should be penalized for hallucination.

---

## 26-graph-ocean-acidification-hawaii.jpg — Ocean acidification chart (Hawaii)

**Multi-series line chart** showing ocean acidification trends from **1958 to 2018**.

**Title**: "Ocean acidification"

**Axes (verified 2026-06-19)**:
- X: "Year" — range **1958 to 2018**
- Left Y: "CO₂" — range **275 to 425** (ppm)
- Right Y: "pH" — range **8.03 to 8.33**

**Data series** (with discrete dots):
- **Atmospheric CO₂** (red): measured at Station Mauna Loa — 1958: ~315 ppm, 2018: ~415 ppm
- **Ocean pCO₂** (green): measured at Station ALOHA — starts 1990 at ~325 ppm, 2018: ~380 ppm
- **Ocean pH** (blue): measured at Station ALOHA — drops from **~8.12 to ~8.06** (taking rough averages; acidification trend)

**Inset map**: Hawaiian Islands with two stations marked (Mauna Loa as red triangle, ALOHA as red dot).

**Source context**: Data aligns with NOAA Mauna Loa Observatory (Keeling Curve) + Hawaii Ocean Time-series (HOT) program at Station ALOHA (running since 1988), but **no source citation is printed on the chart**.

---

## 27-graph-carbonate-based-marine-life-survival-against-ph.jpeg — Carbonate marine life survival vs pH

**Dual y-axis line graph** showing the relationship between ocean pH and marine life survival from **1940 to 2100** (historical + projected).

**Title**: "Carbonate based marine life survival against pH"

**Axes**:
- X: "YEAR" (1940-2100)
- Left Y: "Marine life percentage survival %"
- Right Y: "pH"

**Data series** (with curve shapes verified 2026-06-19):
- **Blue** "Plankton Fish, Whales & carbonate based life" — **almost linear descending**, almost bottoms around **2049**, completely bottoms out in **2060**, ends at 0%
- **Yellow** "bacteria, toxic dinoflagellate phytoplankton" — **rises exponentially**, maxes out around **2044**, then goes **flat** at maximum
- **Red** "pH" — **almost linear decrease**, starting at **8.2 pH in 1940**, ending at **7.9 pH in 2070**

**Key data points**:
- 1940: 100% blue survival, pH 8.2
- 2020: 50% blue survival (annotated "Current position, 50% of all marine life lost since 1950"), pH ~8.05
- ~**2042**: tipping point (annotated)
- ~**2044**: yellow curve maxes out (flat)
- ~**2049**: blue curve almost bottoms
- ~**2060**: blue curve completely bottoms out (0%)
- ~**2070**: red pH curve ends at 7.9

**Annotations (verbatim — note: 5th annotation contains original typo "stravation" instead of "starvation")**:
1. "Current position, 50% of all marine life lost since 1950"
2. "atmosphere starts to become toxic due to dinoflagellate toxins"
3. "Domoic acid produced by dinoflagellate starts to cause brain damage in whales, seals, birds, sea otters and most marine birds and mammals, has not started"
4. "The tipping point, or point of no return for carbonate marine life"
5. "All marine life and plankton based on carbonate dissolves, along with the loss of all the whales, and fish. We now have run-away climate change and mass stravation of billions of people."

---

## 28-xray-ribfracture-lowerright.jpg — PA Chest X-ray with left 10th posterior rib fracture

**PA (Posteroanterior) chest X-ray** showing a **rib fracture on the right side of the image** (which corresponds to the patient's LEFT side per standard radiology orientation convention).

**Visible anatomy**: Ribs (anterior and posterior), clavicles, thoracic spine, scapulae, lungs (radiolucent), heart silhouette, diaphragm, soft tissue.

**Fracture finding (verified 2026-06-19)**:
- Location: **left 10th posterior rib** (patient's left = right side of displayed image)
- Appearance: distinct lucent (dark) line through the rib
- **No significant displacement or angulation**
- No pneumothorax, no pleural effusion, costophrenic angles sharp

**Other findings**: None — no medical devices (lines/tubes), no soft tissue abnormalities, no image artifacts.

**Image metadata**: Well-exposed, good contrast. **No patient ID markers, no date stamps, no technical annotations visible**.

**Note for judging**: The "lower-right" in the filename refers to the right side of the displayed image. Models correctly identifying the fracture location should describe it as either:
- "right side of the image" (visually accurate), OR
- "patient's left side" (anatomically accurate using radiology convention)

Both phrasings are correct. Models placing the fracture on the patient's right side (left of image) are wrong.

---

## 29-painting-hundertwasser.jpeg — Hundertwasser painting (1965)

**Painting BY Friedensreich Hundertwasser** (Austrian artist, 1928-2000), dated **December 1965** (signature shows year "1965" and possibly month "12" in a red/pink box, otherwise illegible).

**Scene (verified 2026-06-19)**:
- **3 buildings** in an imaginary cityscape
- **Leftmost**: red circular tall tower with pointy roof
- **Two right buildings**: yellow-themed, **open at top (no roofs), nothing inside** (visible interior)
- **Dark ground** at the bottom (NOT water)
- **Blue horizon and sky** above (NOT a water scene)

**Hallmark style elements**:
- **No straight lines** — all edges organic and curved
- Bright saturated colors with **dominant yellow**
- **Onion domes** in multiple colors (red, green, white)
- **Embedded human face** in the **lower-left corner of the rightmost building** (feminine, very white skin, **2 red dots on the cheeks** — signature Hundertwasser motif)
- **Blue teardrop shapes** inside most windows (NOT eye-shaped windows — the teardrops are a separate motif)
- **Windows have various shapes**: arched, rectangular with top decoration
- Layered color application, rejection of traditional linear perspective

**Signature**: Red/pink box, mostly illegible except for "**1965**" (year) and possibly "**12**" (month, suggesting December 1965).

---

## 30-where-is-waldo.webp — Classic "Where's Waldo" beach scene

**Original classic "Where's Waldo" / "Where's Wally"** illustration by **Martin Handford** (not fan art) — dense crowd beach scene.

**Setting**: Crowded **sunny beach**.

**Composition**:
- Foreground: vast yellow sand beach packed with hundreds of tiny figures
- Background: blue sea with multiple boats/ships (verified 2026-06-19)
- Sky: light blue with white clouds

**Beach huts (verified 2026-06-19)**:
- **5 blue-and-white striped beach huts**: 3 on the left, 2 on the right
- **1 red-and-white striped beach hut** on the right

**Watercraft (verified 2026-06-19; refined 2026-06-21)**:
- **4 sailboats**: 2 white sails, 1 pink sail, 1 half-yellow/half-white sail (verified 2026-06-21)
- **3 steamships** (also called "tugboats" or "steam boats" by some models — acceptable)
- **1 outboard motorboat with waterskier**
- **1 submarine**
- **1 inflatable dinghy**
- **1 cruise ship** (some models call it "ferry" or "tour boat" — acceptable)
- **2 cargo ships**
- **1 large passenger hovercraft** all the way to the right — blue and white (verified 2026-06-21)
- Possibly 1 more sailboat on the horizon

**Distinctive shore elements**:
- Numerous **beach umbrellas** (various colors)
- **Beach chairs / loungers** (verified 2026-06-21)
- **Tents** (separate from beach huts; verified 2026-06-21)
- **Coolers / picnic baskets** (verified 2026-06-21)
- **Balloons** (verified 2026-06-21)
- **Volleyball** being played (verified 2026-06-21)
- **Surfboards / bodyboards** (verified 2026-06-21)
- **Human pyramid in the water** (verified 2026-06-21)
- **Ice cream cart** on the beach (verified 2026-06-21)
- **Punch and Judy booth** (NOT a food stall; verified 2026-06-21)
- Windbreakers visible
- **6 horses near the water** (verified 2026-06-20) — image owner confirmed these are present; do NOT mark as hallucination
- Activities: swimming, playing with balls, building sandcastles, reading, walking dogs
- Possibly present (owner cannot confirm with certainty): clown costume person, person doing handstand

**NOT in image (often hallucinated)**:
- No pier or jetty — models describing one are confusing the hovercraft for a pier (verified 2026-06-21)
- No carriage pulled by horses
- No lighthouse or tower in the distance

**Hidden characters (verified 2026-06-19)**:
- **Waldo** — red-and-white horizontally striped shirt, brown pants, glasses, bobble hat — located **on the left of the red-and-white striped beach hut, just above the first windbreaker**
- **Odlaw** — Waldo's nemesis, **striped** (owner uncertain whether black-and-white or yellow-and-black — both are valid descriptions; the canonical Odlaw color scheme is yellow/black) — peeking out from **behind the second beach hut from the left**
- **Wizard Whitebeard** — long white beard, hat — **above the first umbrella from the left, on the same row as the huts, approximately on the 1/4 vertical line**
- **Wenda** — long brown hair, red-and-white striped dress — **NOT FOUND** by image owner (may or may not be present)
- **Woof** (the dog) — **NOT FOUND** by image owner (may or may not be present)

**Style**: Authentic original Martin Handford illustration — intricate, dense, hand-painted detail.

**Note for judging**:
- Models that find Waldo/Odlaw/Wizard Whitebeard should get strong credit
- Models claiming to find Wenda or Woof should be verified carefully — the image owner could not locate them
- Models describing the scene as fan art / not original Handford are wrong
- Models calling the artist "Dick Bruna" are WRONG — it's Martin Handford
- Models claiming Waldo is at "bottom center with blue shorts" are WRONG — he's at "left of red-and-white hut, brown pants"
- Models describing a pier/jetty are confusing the hovercraft for a pier — penalize
