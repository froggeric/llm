#!/usr/bin/env python3
"""
VLM Quality Scorecard v5 — hybrid deterministic + LLM-judge.

Extends v4 with deterministic probes for images 21-30 (new categories:
medical, scientific graphs, fine art, dense scenes, animation, motion blur).

v5 = v4 unchanged for images 01-20 + new probes for 21-30.

Three layers (same as v4):
  1. Failure-mode detector: empty / truncated / repetition_loop / normal
  2. Deterministic probes: verifiable facts only
  3. LLM-judge: per-image holistic scoring
"""
import json
import re
import argparse
from collections import defaultdict
from pathlib import Path
from rapidfuzz import fuzz

RAW = Path("benchmark-results/raw.jsonl")
GROUND_TRUTH = Path("test-images/GROUND-TRUTH.md")
JUDGE_DIR = Path("benchmark-results/judgments_v5")
JUDGE_DIR.mkdir(parents=True, exist_ok=True)


# ============================================================================
# Layer 1: Failure-mode detection
# ============================================================================

def detect_failure_mode(record):
    """Returns one of: 'empty', 'truncated', 'repetition_loop', 'normal'."""
    content = record.get("content", "") or ""
    eval_count = record.get("eval_count", 0)
    finish_reason = record.get("finish_reason", "")
    max_budget = record.get("max_tokens_budget", 4096)  # old runs had 4096

    if len(content.strip()) == 0:
        # Distinguish "thought forever, produced nothing" from genuine empty
        if eval_count >= 100 or finish_reason == "length":
            return "truncated"
        return "empty"

    # Truncation: hit cap mid-output (with some output produced)
    if finish_reason == "length" or eval_count >= max_budget - 5:
        return "truncated"

    # Repetition loop: any 10-word window appearing >= 5 times
    words = content.split()
    if len(words) >= 50:
        chunks = [" ".join(words[i:i+10]) for i in range(0, len(words) - 10, 5)]
        if chunks:
            from collections import Counter
            most_common_count = Counter(chunks).most_common(1)[0][1]
            if most_common_count >= 5:
                return "repetition_loop"

    return "normal"


# ============================================================================
# Layer 2: Deterministic probes — verifiable facts only
# ============================================================================

def fuzzy_contains(text, target, max_distance=1):
    """True if `target` appears in `text` with edit distance <= max_distance."""
    text_lower = text.lower()
    target_lower = target.lower()
    if target_lower in text_lower:
        return True
    # Sliding window fuzzy match
    target_words = target_lower.split()
    n = len(target_words)
    if n == 0:
        return False
    text_words = text_lower.split()
    for i in range(len(text_words) - n + 1):
        window = " ".join(text_words[i:i+n])
        if fuzz.ratio(window, target_lower) >= 90:  # ~1 edit per 10 chars
            return True
    return False


def probe_explicit_count(text, valid_values, wrong_values=None):
    """Detect explicit count claim like '11 panels' or 'five people'.
    Returns 'hit', 'wrong', or 'miss' (no explicit claim).
    ONLY matches when number is adjacent to the noun — no narrative regex.
    """
    wrong_values = wrong_values or []
    # Pattern: number (word or digit) immediately before or after noun
    # Limited to actual count claims, not article "a" + noun
    for v in valid_values:
        for form in [str(v), _number_to_word(v)]:
            if form is None:
                continue
            # Look for "(number) (noun)" pattern within 1-2 words
            for noun in ['panels', 'people', 'persons', 'individuals', 'adults',
                         'men', 'women', 'figures', 'surfers', 'monks', 'disciples',
                         'artists', 'albums', 'colors', 'colours', 'classes']:
                pattern = rf'\b{form}\s+(?:[a-z]+\s+){{0,1}}{noun}\b'
                if re.search(pattern, text.lower()):
                    return 'hit'
                pattern_rev = rf'\b{noun}\s+(?:[a-z]+\s+){{0,1}}{form}\b'
                if re.search(pattern_rev, text.lower()):
                    return 'hit'
    for v in wrong_values:
        for form in [str(v), _number_to_word(v)]:
            if form is None:
                continue
            for noun in ['panels', 'people', 'persons', 'individuals', 'adults',
                         'men', 'women', 'figures', 'surfers', 'monks', 'disciples',
                         'artists', 'albums', 'colors', 'colours', 'classes']:
                pattern = rf'\b{form}\s+(?:[a-z]+\s+){{0,1}}{noun}\b'
                if re.search(pattern, text.lower()):
                    return 'wrong'
    return 'miss'


def _number_to_word(n):
    """1 -> 'one', 2 -> 'two', ..., else None."""
    mapping = {1: 'one', 2: 'two', 3: 'three', 4: 'four', 5: 'five',
               6: 'six', 7: 'seven', 8: 'eight', 9: 'nine', 10: 'ten',
               11: 'eleven', 12: 'twelve', 13: 'thirteen', 14: 'fourteen', 15: 'fifteen'}
    return mapping.get(n)


def probe_text_present(text, target, fuzzy=True):
    """Check if target text appears in response (optionally fuzzy)."""
    if fuzzy:
        return fuzzy_contains(text, target)
    return target.lower() in text.lower()


def probe_hallucinated(text, hallucinated_strings):
    """Check for any hallucinated exact string."""
    found = []
    for h in hallucinated_strings:
        if h.lower() in text.lower():
            found.append(h)
    return found


# ============================================================================
# Per-image probe definitions
# ============================================================================

def probes_for_image(image_name, content):
    """Returns dict of probe results for a single response.
    Each value is 'hit' / 'miss' / 'wrong' / list of hallucinations.
    """
    p = {}

    if image_name == "01_ui_login.png":
        p["branding"] = probe_text_present(content, "sign in to acme")
        p["email"] = probe_text_present(content, "user@example.com")
        p["error_msg"] = probe_text_present(content, "invalid credentials")
        p["checkbox"] = probe_text_present(content, "keep me signed in")
        p["forgot"] = probe_text_present(content, "forgot your password")

    elif image_name == "02_code_python.png":
        p["fastapi_import"] = probe_text_present(content, "from fastapi import fastapi")
        p["basemodel"] = probe_text_present(content, "class user(basemodel)")
        p["post_route"] = probe_text_present(content, "@app.post")
        p["commit"] = probe_text_present(content, "db.commit")

    elif image_name == "03_error_trace.png":
        p["psycopg2"] = probe_text_present(content, "psycopg2")
        p["operational"] = probe_text_present(content, "operational")
        p["auth_failed"] = probe_text_present(content, "password authentication failed")
        p["host"] = probe_text_present(content, "db.internal")
        p["port"] = probe_text_present(content, "10.0.0.5")
        p["main_line"] = probe_text_present(content, "main.py")
        p["hallucinated_db"] = probe_hallucinated(content, ["psycopgopg", "postgres internal"])

    elif image_name == "04_architecture.png":
        for fact in ["web client", "api gateway", "fastapi", "auth service",
                     "worker queue", "postgres", "redis"]:
            p[f"comp_{fact}"] = probe_text_present(content, fact)
        p["https"] = probe_text_present(content, "https")
        p["grpc"] = probe_text_present(content, "grpc")

    elif image_name == "05-cropped-youtube-capture.png":
        p["shaolin"] = probe_text_present(content, "shaolin") or probe_text_present(content, "monk", fuzzy=False)
        p["qigong"] = (probe_text_present(content, "qigong") or
                       probe_text_present(content, "qi gong") or
                       probe_text_present(content, "yi jin jing"))
        p["temple"] = (probe_text_present(content, "temple") or
                       probe_text_present(content, "shrine"))

    elif image_name == "06-trading-card.jpg":
        p["card"] = (probe_text_present(content, "trading card") or
                     probe_text_present(content, "card") or
                     probe_text_present(content, "collectible"))
        p["pacman"] = (probe_text_present(content, "pacman") or
                       probe_text_present(content, "pac-man") or
                       probe_text_present(content, "arcade"))
        p["yinyang"] = (probe_text_present(content, "yin-yang") or
                        probe_text_present(content, "yin yang") or
                        probe_text_present(content, "yin-yang") or
                        probe_text_present(content, "taichi") or
                        probe_text_present(content, "taiji"))
        p["ghost"] = (probe_text_present(content, "ghost") or
                      probe_text_present(content, "blinky"))
        p["popart"] = (probe_text_present(content, "pop art") or
                       probe_text_present(content, "pop-art") or
                       probe_text_present(content, "vibrant") or
                       probe_text_present(content, "bold"))

    elif image_name == "07-photo-massage-therapists.jpg":
        p["count"] = probe_explicit_count(content, [5], [4, 6, 7])
        p["massage"] = probe_text_present(content, "massage") or probe_text_present(content, "therapy")
        p["bamboo"] = probe_text_present(content, "bamboo")

    elif image_name == "08-poster-class-schedule.jpg":
        p["brand"] = probe_text_present(content, "vic health club")
        for act in ["yoga", "shaolin", "fitness", "qigong", "taiji", "calisthenics"]:
            p[f"act_{act}"] = probe_text_present(content, act)
        p["url"] = probe_text_present(content, "vichealth.club")

    elif image_name == "09-poster-artnight.jpg":
        p["artnight"] = probe_text_present(content, "artnight") or probe_text_present(content, "art night")
        p["acrylic"] = probe_text_present(content, "acrylic")
        p["vic_art"] = probe_text_present(content, "vic art club") or probe_text_present(content, "art club")
        p["vic_health"] = probe_text_present(content, "vic health club")

    elif image_name == "10-photo-rice-porridge-breakfast.png":
        p["porridge"] = (probe_text_present(content, "porridge") or
                         probe_text_present(content, "congee"))
        p["egg"] = probe_text_present(content, "egg")
        p["runny"] = probe_text_present(content, "runny") or probe_text_present(content, "yolk")
        p["you_tiao"] = (probe_text_present(content, "you tiao") or
                         probe_text_present(content, "you char") or
                         probe_text_present(content, "fried dough") or
                         probe_text_present(content, "chinese donut"))
        p["soy_milk"] = probe_text_present(content, "soy milk") or probe_text_present(content, "soybean")
        p["red_plate"] = "red" in content.lower() and "plate" in content.lower()
        p["marble"] = probe_text_present(content, "marble")

    elif image_name == "11-collage-therapist-photo.jpg":
        p["two_photos"] = (probe_text_present(content, "two") or
                           probe_text_present(content, "side-by-side") or
                           probe_text_present(content, "side by side") or
                           probe_text_present(content, "diptych"))
        p["massage"] = probe_text_present(content, "massage") or probe_text_present(content, "therapy")
        p["bamboo"] = probe_text_present(content, "bamboo") or probe_text_present(content, "thai")

    elif image_name == "12-manga-nausicaa-colour.png":
        p["count"] = probe_explicit_count(content, [11], [10, 12, 9, 13])
        p["nausicaa"] = probe_text_present(content, "nausicaa") or "nausicaä" in content.lower()
        p["ghibli"] = (probe_text_present(content, "miyazaki") or
                       probe_text_present(content, "ghibli"))
        p["ohm"] = (probe_text_present(content, "ohm") or
                    probe_text_present(content, "ohmu"))

    elif image_name == "13-colour-swatch-nausicaa.jpg":
        p["palette"] = probe_text_present(content, "palette") or probe_text_present(content, "swatch")
        p["nausicaa"] = probe_text_present(content, "nausicaa") or "nausicaä" in content.lower()
        p["caption"] = probe_text_present(content, "the colors of") or probe_text_present(content, "the colours of")
        p["count"] = probe_explicit_count(content, [10], [8, 9, 11, 12])

    elif image_name == "13-marker-drawing-spotify-cassette.jpeg":
        p["marker"] = probe_text_present(content, "marker")
        p["cassette"] = probe_text_present(content, "cassette") or probe_text_present(content, "tape")
        p["musique"] = probe_text_present(content, "musique")
        p["volonte"] = probe_text_present(content, "volonté") or probe_text_present(content, "volonte")

    elif image_name == "14-screenshot-onerpm-catalog.jpg":
        # Hard titles (thumbnail-only)
        for title in ["secreto del sur", "sol sol", "ocean flute",
                      "spirits of the wind", "timeless whispers"]:
            key = f"hard_{title[:8].replace(' ', '_')}"
            p[key] = fuzzy_contains(content, title, max_distance=1)
        # Visible titles (worth less)
        for title in ["winding down", "silk and steel", "waikiki",
                      "gaelic cradle song", "endless ocean"]:
            key = f"easy_{title[:8].replace(' ', '_')}"
            p[key] = fuzzy_contains(content, title, max_distance=1)
        # Status tags
        p["status_distributed"] = probe_text_present(content, "distributed")
        p["status_incomplete"] = probe_text_present(content, "incomplete")
        p["status_approved"] = probe_text_present(content, "approved")
        # Hallucinated titles
        p["hallucinated"] = probe_hallucinated(content, [
            "galactic cradle song", "wakkie", "tejido centro",
            "eindlea", "numbo", "ocean flutes", "timeless adventures",
            "onetruemusic",  # hallucinated platform name (real is ONErpm)
            "presque louis by frederic"  # check if confused
        ])

    elif image_name == "16-album-cover-lofi.jpg":
        # Phrase should appear at least once
        p["phrase_once"] = "songs to stare at the ceiling to" in content.lower()
        # True repetition (5+ times) is the real failure, not 2-3 mentions
        p["phrase_count"] = content.lower().count("songs to stare at the ceiling to")
        p["window"] = probe_text_present(content, "window")
        p["ceiling"] = probe_text_present(content, "ceiling")
        p["lofi"] = (probe_text_present(content, "lo-fi") or
                     probe_text_present(content, "lofi") or
                     probe_text_present(content, "vaporwave") or
                     probe_text_present(content, "film grain") or
                     probe_text_present(content, "grain"))

    elif image_name == "17-logo-vic-health-club.png":
        p["brand"] = probe_text_present(content, "vic health club")
        p["heart"] = probe_text_present(content, "heart")
        p["hands"] = probe_text_present(content, "hand") or probe_text_present(content, "palm")
        p["laurel"] = (probe_text_present(content, "laurel") or
                       probe_text_present(content, "wreath") or
                       probe_text_present(content, "branch"))
        p["monochrome"] = (probe_text_present(content, "black and white") or
                           probe_text_present(content, "monochrome"))

    elif image_name == "18-qrcode-vic-health-club.png":
        p["qr"] = probe_text_present(content, "qr code") or probe_text_present(content, "qr")
        p["scan"] = probe_text_present(content, "scan")
        p["url"] = probe_text_present(content, "vichealth.club")

    elif image_name == "19-watercolour-painting-surfers-fuerteventura.png":
        # Critical: did model see 2 surfers? Judge this, don't regex it.
        # Deterministic check: only explicit "two/2/both" + (surfer/person/figure) within 30 chars
        t = content.lower()
        p["explicit_two"] = bool(re.search(
            r'\b(?:two|2|both)\b.{0,30}(?:surfer|person|figure|man|rider|individual)', t))
        p["watercolor"] = (probe_text_present(content, "watercolor") or
                           probe_text_present(content, "watercolour"))
        p["waves"] = probe_text_present(content, "wave")
        p["purple_mtn"] = (probe_text_present(content, "purple") or
                           probe_text_present(content, "lavender") or
                           probe_text_present(content, "violet")) and probe_text_present(content, "mountain")
        p["wetsuit"] = probe_text_present(content, "wetsuit") or probe_text_present(content, "wet suit")
        p["rocks"] = probe_text_present(content, "rock")

    elif image_name == "20-ultrawide-kung-fu-banner-with-logo.png":
        p["shaolin_chinese"] = "少林寺" in content
        p["shaolin_temple"] = probe_text_present(content, "shaolin temple")
        p["austria"] = (probe_text_present(content, "öster") or
                        probe_text_present(content, "osterr") or
                        probe_text_present(content, "austria"))
        p["count_people"] = probe_explicit_count(content, [9], [7, 8, 10, 11])
        p["heart"] = probe_text_present(content, "heart")
        p["hands"] = probe_text_present(content, "hand") or probe_text_present(content, "palm")
        p["laurel"] = (probe_text_present(content, "laurel") or
                       probe_text_present(content, "wreath"))
        p["weapon"] = (probe_text_present(content, "sword") or
                       probe_text_present(content, "blade") or
                       probe_text_present(content, "weapon") or
                       probe_text_present(content, "shield") or
                       probe_text_present(content, "staff") or
                       probe_text_present(content, "lance") or
                       probe_text_present(content, "spear"))

    # ========================================================================
    # v5: Images 21-30 (new categories — medical, scientific, art, dense scenes)
    # ========================================================================

    elif image_name == "21-motion-blur.jpg":
        # Long-exposure Hong Kong street scene
        p["long_exposure"] = (probe_text_present(content, "long exposure") or
                              probe_text_present(content, "long-exposure") or
                              probe_text_present(content, "time exposure") or
                              probe_text_present(content, "long shutter") or
                              probe_text_present(content, "slow shutter"))
        p["light_trails"] = (probe_text_present(content, "light trail") or
                             probe_text_present(content, "vehicle light") or
                             probe_text_present(content, "light streak"))
        p["tram"] = probe_text_present(content, "tram")
        p["bus_stop"] = (probe_text_present(content, "bus stop") or
                         probe_text_present(content, "kiosk") or
                         probe_text_present(content, "bus shelter"))
        p["dah_sing"] = (probe_text_present(content, "dah sing") or
                         "大新" in content or
                         probe_text_present(content, "ta hing"))
        p["dvfx"] = (fuzzy_contains(content, "dvfx", max_distance=1) or
                     fuzzy_contains(content, "dmfx", max_distance=1))
        p["hong_kong"] = probe_text_present(content, "hong kong") or probe_text_present(content, "hk ")
        p["night"] = probe_text_present(content, "night")
        p["crosswalk"] = (probe_text_present(content, "pedestrian crossing") or
                          probe_text_present(content, "crosswalk") or
                          probe_text_present(content, "zebra crossing"))
        p["hallucinated_mtr"] = probe_hallucinated(content, ["MTR sign", "MTR logo"])

    elif image_name == "22-spritesheet-bubble-bobble.png":
        # Bubble Bobble fan spritesheet
        p["title"] = probe_text_present(content, "bubble bobble")
        p["credit_125scratch"] = (fuzzy_contains(content, "125scratch", max_distance=2) or
                                  fuzzy_contains(content, "125 scratch", max_distance=2))
        p["pac_man_red"] = (probe_text_present(content, "pac-man red") or
                            probe_text_present(content, "pac man red") or
                            probe_text_present(content, "pacman red"))
        p["atariage"] = (probe_text_present(content, "atariage") or
                         probe_text_present(content, "atari age"))
        p["taito"] = probe_text_present(content, "taito")
        p["extend"] = probe_text_present(content, "extend")
        p["hurry_up"] = probe_text_present(content, "hurry up")
        p["help"] = probe_text_present(content, "help!!") or probe_text_present(content, "help!")
        p["round"] = probe_text_present(content, "round")
        # Verified 2026-06-21: Bub is green dragon (player 1), Bob is blue/purple (player 2)
        p["green_dragon_bub"] = (("green" in content.lower() and "dragon" in content.lower()) or
                                 probe_text_present(content, "bub"))
        p["ghost_enemies"] = probe_text_present(content, "ghost")
        # Specific food items (verified 2026-06-21)
        p["item_watermelon"] = probe_text_present(content, "watermelon")
        p["item_banana"] = probe_text_present(content, "banana")
        p["item_cake"] = (probe_text_present(content, "cake slice") or
                          probe_text_present(content, "cake"))
        p["item_ice_cream"] = probe_text_present(content, "ice cream")
        p["item_apple"] = probe_text_present(content, "apple")
        p["item_mushroom"] = probe_text_present(content, "mushroom")
        p["hallucinated_d2sereno"] = probe_hallucinated(content, [
            "d2sereno", "d2 serotonin", "happy well", "i'm happy to share",
            "i am happy to share", "pink cumin", "super dragon boss",
            "red dragon boss"
        ])

    elif image_name == "23-animation.jpg":
        # 6-frame blue cat RUN cycle
        p["grid"] = (probe_text_present(content, "2x3") or
                     probe_text_present(content, "2 x 3") or
                     probe_text_present(content, "3x2") or
                     probe_text_present(content, "six frame") or
                     probe_text_present(content, "6 frame") or
                     probe_text_present(content, "six-frame") or
                     probe_text_present(content, "6-frame"))
        p["blue_cat"] = ("blue" in content.lower() and
                         ("cat" in content.lower() or "feline" in content.lower()))
        p["run_cycle"] = (probe_text_present(content, "run cycle") or
                          probe_text_present(content, "running cycle") or
                          probe_text_present(content, "run loop") or
                          probe_text_present(content, "running animation") or
                          probe_text_present(content, "running loop"))
        p["stride"] = probe_text_present(content, "stride")
        p["loop"] = probe_text_present(content, "loop")
        p["frame_numbers"] = (probe_text_present(content, "frame 1") or
                              probe_text_present(content, "frame1") or
                              probe_text_present(content, "numbered"))
        p["flat_shadow"] = (probe_text_present(content, "shadow") and
                            (probe_text_present(content, "flat") or
                             probe_text_present(content, "simple")))

    elif image_name == "24-lymphatic-system-handdrawn.jpg":
        # Hand-drawn lymphatic system diagram with color-coded labels
        p["title"] = (probe_text_present(content, "lymphatic system") or
                      probe_text_present(content, "human lymphatic"))
        # 6 organs
        for organ in ["tonsils", "thymus", "spleen"]:
            p[f"organ_{organ}"] = probe_text_present(content, organ)
        p["vessels"] = (probe_text_present(content, "lymph vessel") or
                        probe_text_present(content, "lymphatic vessel") or
                        probe_text_present(content, "lymph vessels"))
        p["nodes"] = (probe_text_present(content, "lymph node") or
                      probe_text_present(content, "lymph nodes"))
        p["marrow"] = (probe_text_present(content, "bone marrow") or
                       probe_text_present(content, "marrow"))
        # Color coding (next-to-organ)
        p["color_tonsils_pink"] = _colors_near(content, "tonsil", ["pink"])
        p["color_thymus_orange"] = _colors_near(content, "thymus", ["orange"])
        p["color_spleen_purple"] = _colors_near(content, "spleen", ["purple", "violet", "lavender"])
        p["color_marrow_yellow"] = _colors_near(content, "marrow", ["yellow"])
        # Functions
        p["func_infection"] = (probe_text_present(content, "infection") or
                               probe_text_present(content, "immune"))
        p["func_fluid"] = (probe_text_present(content, "lymph fluid") or
                           probe_text_present(content, "carries lymph"))
        p["func_filter"] = (probe_text_present(content, "filter") or
                            probe_text_present(content, "filters harmful"))
        p["lined_paper"] = (probe_text_present(content, "lined paper") or
                            probe_text_present(content, "notebook paper") or
                            probe_text_present(content, "notebook"))

    elif image_name == "25-mri-brain-parkinson.jpeg":
        # 2x3 grid of brain MRI slices
        p["mri"] = (probe_text_present(content, "mri") or
                    probe_text_present(content, "magnetic resonance"))
        p["grid"] = (probe_text_present(content, "2x3") or
                     probe_text_present(content, "2 x 3") or
                     probe_text_present(content, "3x2") or
                     probe_text_present(content, "six") or
                     probe_text_present(content, "6 slices") or
                     probe_text_present(content, "six slices"))
        # Three anatomical planes
        for plane in ["coronal", "sagittal", "axial"]:
            p[f"plane_{plane}"] = probe_text_present(content, plane)
        # Orientation markers
        p["marker_r"] = bool(re.search(r'\bR\b', content))
        p["marker_al"] = bool(re.search(r'\bAL\b', content))
        p["marker_mf"] = bool(re.search(r'\bMF\b', content))
        # Scale
        p["scale_5cm"] = (probe_text_present(content, "5cm") or
                          probe_text_present(content, "5 cm") or
                          probe_text_present(content, "5-cm"))
        # Hallucination: claims to see specific lesions/annotations
        p["hallucinated_lesions"] = probe_hallucinated(content, [
            "substantia nigra", "lesion", "tumor", "infarct", "stroke",
            "annotated", "arrow pointing"
        ])

    elif image_name == "26-graph-ocean-acidification-hawaii.jpg":
        # 3-series line chart, Hawaii stations
        p["title"] = probe_text_present(content, "ocean acidification")
        # Three series
        p["series_atmos_co2"] = (probe_text_present(content, "atmospheric co2") or
                                 probe_text_present(content, "atmospheric co₂") or
                                 probe_text_present(content, "atmospheric carbon"))
        p["series_ocean_pco2"] = (probe_text_present(content, "ocean pco2") or
                                  probe_text_present(content, "seawater pco2") or
                                  probe_text_present(content, "ocean pco₂"))
        p["series_ph"] = (probe_text_present(content, "ph") and
                          "ocean" in content.lower())
        # Date range
        p["year_1958"] = "1958" in content
        p["year_2018"] = "2018" in content
        # Stations (rare OCR)
        p["mauna_loa"] = probe_text_present(content, "mauna loa")
        p["aloha"] = probe_text_present(content, "aloha")
        p["hawaii"] = (probe_text_present(content, "hawaii") or
                       probe_text_present(content, "hawaiian"))

    elif image_name == "27-graph-carbonate-based-marine-life-survival-against-ph.jpeg":
        # Carbonate marine life survival vs pH (1940-2100)
        p["title"] = (probe_text_present(content, "carbonate") and
                      probe_text_present(content, "marine life"))
        # Three series
        p["plankton"] = probe_text_present(content, "plankton")
        p["dinoflagellate"] = (probe_text_present(content, "dinoflagellate") or
                               probe_text_present(content, "phytoplankton"))
        p["ph_series"] = probe_text_present(content, "ph")
        # X-axis start (common error: 1950 not 1940)
        p["year_1940"] = "1940" in content
        p["year_2100"] = "2100" in content
        # Annotations
        p["tipping_point"] = probe_text_present(content, "tipping point")
        p["current_position"] = probe_text_present(content, "current position")
        p["domoic_acid"] = (probe_text_present(content, "domoic acid") or
                            probe_text_present(content, "domoic"))
        p["starvation"] = (probe_text_present(content, "starvation") or
                           probe_text_present(content, "stravation"))
        # Hallucination: "atomic acid"
        p["hallucinated_atomic"] = probe_hallucinated(content, ["atomic acid"])

    elif image_name == "28-xray-ribfracture-lowerright.jpg":
        # PA Chest X-ray with left 10th posterior rib fracture
        p["modality"] = (probe_text_present(content, "x-ray") or
                         probe_text_present(content, "x ray") or
                         probe_text_present(content, "radiograph") or
                         probe_text_present(content, "chest x-ray") or
                         probe_text_present(content, "cxr"))
        p["view_pa"] = (probe_text_present(content, "pa ") or
                        probe_text_present(content, "pa view") or
                        probe_text_present(content, "posteroanterior") or
                        probe_text_present(content, "frontal"))
        # KEY FINDING
        p["fracture"] = (probe_text_present(content, "fracture") or
                         probe_text_present(content, "break") or
                         probe_text_present(content, "broken rib") or
                         probe_text_present(content, "lucent line"))
        p["rib"] = probe_text_present(content, "rib")
        p["rib_10th"] = (probe_text_present(content, "10th rib") or
                         probe_text_present(content, "tenth rib") or
                         probe_text_present(content, "10th posterior") or
                         probe_text_present(content, "tenth posterior"))
        p["posterior"] = probe_text_present(content, "posterior")
        p["location_left"] = (probe_text_present(content, "patient's left") or
                              probe_text_present(content, "patient left") or
                              probe_text_present(content, "left side") or
                              probe_text_present(content, "right side of the image") or
                              probe_text_present(content, "right of the image"))
        p["no_pneumothorax"] = (probe_text_present(content, "no pneumothorax") or
                                probe_text_present(content, "without pneumothorax"))
        p["no_displacement"] = (probe_text_present(content, "no displacement") or
                                probe_text_present(content, "no significant displacement") or
                                probe_text_present(content, "without displacement"))

    elif image_name == "29-painting-hundertwasser.jpeg":
        # Hundertwasser painting (1965)
        p["artist"] = probe_text_present(content, "hundertwasser")
        p["year_1965"] = "1965" in content
        # Architecture
        p["buildings_3"] = (probe_text_present(content, "three buildings") or
                            probe_text_present(content, "3 buildings") or
                            probe_text_present(content, "three structures") or
                            probe_text_present(content, "3 structures"))
        p["red_tower"] = (probe_text_present(content, "red") and
                          (probe_text_present(content, "tower") or
                           probe_text_present(content, "spire")))
        p["yellow"] = probe_text_present(content, "yellow")
        p["onion_dome"] = (probe_text_present(content, "onion dome") or
                           probe_text_present(content, "onion-shaped") or
                           probe_text_present(content, "dome"))
        # Face with red cheeks (signature motif)
        p["embedded_face"] = probe_text_present(content, "face")
        p["red_cheeks"] = ((probe_text_present(content, "red") or
                            probe_text_present(content, "rosy") or
                            probe_text_present(content, "freckle")) and
                           (probe_text_present(content, "cheek") or
                            probe_text_present(content, "freckle")))
        p["blue_teardrop"] = (probe_text_present(content, "teardrop") or
                              probe_text_present(content, "tear-drop") or
                              probe_text_present(content, "tear drop"))
        # Hallucination: describing the dark ground as water
        p["hallucinated_water"] = probe_hallucinated(content, [
            "water scene", "sea in the background", "ocean in the background",
            "river below", "reflecting water"
        ])

    elif image_name == "30-where-is-waldo.webp":
        # Where's Waldo beach scene
        p["waldo_named"] = (probe_text_present(content, "waldo") or
                            probe_text_present(content, "wally"))
        p["beach"] = probe_text_present(content, "beach")
        p["crowd"] = (probe_text_present(content, "crowd") or
                      probe_text_present(content, "hundreds") or
                      probe_text_present(content, "dozens"))
        p["beach_hut"] = probe_text_present(content, "beach hut")
        p["striped"] = probe_text_present(content, "stripe")
        p["blue_white_stripes"] = (("blue" in content.lower() and "white" in content.lower()) and
                                   "stripe" in content.lower())
        p["red_white_stripes"] = (("red" in content.lower() and "white" in content.lower()) and
                                  "stripe" in content.lower())
        p["boat"] = (probe_text_present(content, "sailboat") or
                     probe_text_present(content, "sail boat") or
                     probe_text_present(content, "ship") or
                     probe_text_present(content, "boat") or
                     probe_text_present(content, "vessel"))
        p["umbrella"] = probe_text_present(content, "umbrella")
        # 6 horses near the water (owner-verified 2026-06-20)
        p["horses"] = probe_text_present(content, "horse")
        # Verified 2026-06-21: many additional elements
        p["hovercraft"] = probe_text_present(content, "hovercraft")
        p["beach_chairs"] = (probe_text_present(content, "beach chair") or
                             probe_text_present(content, "lounger") or
                             probe_text_present(content, "lounge chair"))
        p["tent"] = probe_text_present(content, "tent")
        p["cooler"] = (probe_text_present(content, "cooler") or
                       probe_text_present(content, "picnic basket") or
                       probe_text_present(content, "picnic"))
        p["balloon"] = probe_text_present(content, "balloon")
        p["volleyball"] = probe_text_present(content, "volleyball")
        p["surfboard"] = (probe_text_present(content, "surfboard") or
                          probe_text_present(content, "bodyboard"))
        p["human_pyramid"] = (probe_text_present(content, "human pyramid") or
                              probe_text_present(content, "pyramid"))
        p["ice_cream_cart"] = (probe_text_present(content, "ice cream cart") or
                               probe_text_present(content, "ice cream"))
        p["punch_judy"] = (probe_text_present(content, "punch") and
                           probe_text_present(content, "judy"))
        # Bonus: actually locating Waldo (rare; just hint to judge)
        p["claims_waldo_location"] = bool(re.search(
            r'waldo.{0,60}(?:located|found|positioned|hidden|visible|near|center|left|right|top|bottom)',
            content.lower()))
        # Hallucinations (verified wrong 2026-06-21)
        p["hallucinated_woof"] = probe_hallucinated(content, [
            "woof is visible", "woof can be seen", "woof appears",
            "his dog woof", "the dog woof"
        ])
        p["hallucinated_dick_bruna"] = probe_hallucinated(content, ["dick bruna"])
        p["hallucinated_pier"] = probe_hallucinated(content, [
            "pier extending", "pier with people", "jetty extending", "wooden pier", "white pier"
        ])
        p["hallucinated_carriage"] = probe_hallucinated(content, [
            "carriage", "horse-drawn carriage", "pulled by two horses"
        ])

    return p


def _colors_near(text, noun, colors, window_words=8):
    """Check if any of `colors` appears within `window_words` of `noun`."""
    text_lower = text.lower()
    words = text_lower.split()
    noun_lower = noun.lower()
    color_set = set(c.lower() for c in colors)
    for i, w in enumerate(words):
        if noun_lower in w:
            lo = max(0, i - window_words)
            hi = min(len(words), i + window_words + 1)
            window = words[lo:hi]
            if any(c in window for c in color_set):
                return True
    return False


def deterministic_score(image_name, probes, failure_mode):
    """Convert probe results into a 0-10 deterministic score.
    Failure modes cap the score."""
    if failure_mode == "empty":
        return 0.0, "empty response"
    if failure_mode == "truncated":
        return 2.0, "truncated (hit max_tokens)"
    if failure_mode == "repetition_loop":
        return 0.5, "degeneration loop"

    score = 0.0
    max_score = 0.0
    notes = []

    if image_name == "01_ui_login.png":
        weights = {"branding": 1.5, "email": 1.0, "error_msg": 1.5,
                   "checkbox": 1.0, "forgot": 1.0}
        for k, w in weights.items():
            max_score += w
            if probes.get(k):
                score += w

    elif image_name == "02_code_python.png":
        weights = {"fastapi_import": 2.0, "basemodel": 2.0,
                   "post_route": 2.0, "commit": 2.0}
        for k, w in weights.items():
            max_score += w
            if probes.get(k):
                score += w

    elif image_name == "03_error_trace.png":
        weights = {"psycopg2": 1.5, "operational": 1.0, "auth_failed": 1.5,
                   "host": 1.0, "port": 1.0, "main_line": 1.0}
        for k, w in weights.items():
            max_score += w
            if probes.get(k):
                score += w
        # Hallucination penalty
        if probes.get("hallucinated_db"):
            score -= 2.0
            notes.append(f"hallucinated: {probes['hallucinated_db']}")

    elif image_name == "04_architecture.png":
        for k, v in probes.items():
            if k.startswith("comp_"):
                max_score += 0.7
                if v:
                    score += 0.7
        for proto in ["https", "grpc"]:
            max_score += 1.0
            if probes.get(proto):
                score += 1.0

    elif image_name == "05-cropped-youtube-capture.png":
        weights = {"shaolin": 2.5, "qigong": 4.0, "temple": 1.5}
        for k, w in weights.items():
            max_score += w
            if probes.get(k):
                score += w

    elif image_name == "06-trading-card.jpg":
        weights = {"card": 2.0, "pacman": 2.5, "yinyang": 3.5, "ghost": 1.0, "popart": 1.0}
        for k, w in weights.items():
            max_score += w
            if probes.get(k):
                score += w

    elif image_name == "07-photo-massage-therapists.jpg":
        if probes.get("count") == "hit":
            score += 5.0
        elif probes.get("count") == "wrong":
            score -= 2.0
            notes.append("wrong people count")
        max_score += 5.0
        for k in ["massage", "bamboo"]:
            max_score += 1.5
            if probes.get(k):
                score += 1.5

    elif image_name == "08-poster-class-schedule.jpg":
        if probes.get("brand"):
            score += 1.5
        max_score += 1.5
        for k, v in probes.items():
            if k.startswith("act_"):
                max_score += 0.5
                if v:
                    score += 0.5
        if probes.get("url"):
            score += 1.0
        max_score += 1.0

    elif image_name == "09-poster-artnight.jpg":
        weights = {"artnight": 1.5, "acrylic": 2.0, "vic_art": 2.5, "vic_health": 1.5}
        for k, w in weights.items():
            max_score += w
            if probes.get(k):
                score += w

    elif image_name == "10-photo-rice-porridge-breakfast.png":
        weights = {"porridge": 1.5, "egg": 1.0, "runny": 1.5, "you_tiao": 1.5,
                   "soy_milk": 1.0, "red_plate": 0.5, "marble": 0.5}
        for k, w in weights.items():
            max_score += w
            if probes.get(k):
                score += w

    elif image_name == "11-collage-therapist-photo.jpg":
        weights = {"two_photos": 1.5, "massage": 1.5, "bamboo": 2.5}
        for k, w in weights.items():
            max_score += w
            if probes.get(k):
                score += w

    elif image_name == "12-manga-nausicaa-colour.png":
        if probes.get("count") == "hit":
            score += 3.0
        elif probes.get("count") == "wrong":
            score -= 1.5
            notes.append("wrong panel count")
        max_score += 3.0
        weights = {"nausicaa": 3.0, "ghibli": 1.0, "ohm": 2.0}
        for k, w in weights.items():
            max_score += w
            if probes.get(k):
                score += w

    elif image_name == "13-colour-swatch-nausicaa.jpg":
        weights = {"palette": 1.5, "nausicaa": 2.0, "caption": 2.5}
        for k, w in weights.items():
            max_score += w
            if probes.get(k):
                score += w
        if probes.get("count") == "hit":
            score += 1.5
        elif probes.get("count") == "wrong":
            score -= 0.5
        max_score += 1.5

    elif image_name == "13-marker-drawing-spotify-cassette.jpeg":
        weights = {"marker": 1.5, "cassette": 1.5, "musique": 3.0, "volonte": 3.0}
        for k, w in weights.items():
            max_score += w
            if probes.get(k):
                score += w

    elif image_name == "14-screenshot-onerpm-catalog.jpg":
        # Hard titles worth most
        for k, v in probes.items():
            if k.startswith("hard_"):
                max_score += 1.0
                if v:
                    score += 1.0
        # Easy titles worth little
        for k, v in probes.items():
            if k.startswith("easy_"):
                max_score += 0.2
                if v:
                    score += 0.2
        # Status tags
        for stat in ["status_distributed", "status_incomplete", "status_approved"]:
            max_score += 0.5
            if probes.get(stat):
                score += 0.5
        # Hallucination penalty
        if probes.get("hallucinated"):
            score -= 1.5
            notes.append(f"hallucinated: {probes['hallucinated']}")

    elif image_name == "16-album-cover-lofi.jpg":
        if probes.get("phrase_once"):
            score += 4.0
        max_score += 4.0
        # Only flag REAL repetition (5+)
        pc = probes.get("phrase_count", 0)
        if pc >= 5:
            score -= 3.0
            notes.append(f"phrase repeated {pc}x (real repetition)")
        for k in ["window", "ceiling", "lofi"]:
            max_score += 1.0
            if probes.get(k):
                score += 1.0

    elif image_name == "17-logo-vic-health-club.png":
        weights = {"brand": 1.5, "heart": 1.5, "hands": 1.5,
                   "laurel": 3.0, "monochrome": 1.0}
        for k, w in weights.items():
            max_score += w
            if probes.get(k):
                score += w

    elif image_name == "18-qrcode-vic-health-club.png":
        weights = {"qr": 4.0, "scan": 1.0, "url": 2.5}
        for k, w in weights.items():
            max_score += w
            if probes.get(k):
                score += w

    elif image_name == "19-watercolour-painting-surfers-fuerteventura.png":
        # NOTE: explicit_two is just a hint; the judge handles the real assessment
        weights = {"watercolor": 1.5, "waves": 1.0, "purple_mtn": 1.5,
                   "wetsuit": 1.5, "rocks": 1.0}
        for k, w in weights.items():
            max_score += w
            if probes.get(k):
                score += w
        if probes.get("explicit_two"):
            score += 1.5
        max_score += 1.5

    elif image_name == "20-ultrawide-kung-fu-banner-with-logo.png":
        weights = {"shaolin_chinese": 2.0, "shaolin_temple": 1.0, "austria": 1.5,
                   "heart": 1.0, "hands": 1.0, "laurel": 1.5, "weapon": 1.5}
        for k, w in weights.items():
            max_score += w
            if probes.get(k):
                score += w
        if probes.get("count_people") == "hit":
            score += 2.0
        elif probes.get("count_people") == "wrong":
            score -= 1.0
            notes.append("wrong people count")
        max_score += 2.0

    # ========================================================================
    # v5: Images 21-30
    # ========================================================================

    elif image_name == "21-motion-blur.jpg":
        weights = {
            "long_exposure": 2.0,    # technique identification
            "light_trails": 1.5,     # key visual feature
            "tram": 1.5,             # distinctive
            "bus_stop": 1.0,         # central feature (commonly missed)
            "dah_sing": 3.0,         # rare OCR (foreign text)
            "dvfx": 2.0,             # rare OCR
            "hong_kong": 1.0,
            "night": 0.5,
            "crosswalk": 0.5,
        }
        for k, w in weights.items():
            max_score += w
            if probes.get(k):
                score += w
        if probes.get("hallucinated_mtr"):
            score -= 1.0
            notes.append("hallucinated MTR signage")

    elif image_name == "22-spritesheet-bubble-bobble.png":
        weights = {
            "title": 2.0,            # "Bubble Bobble"
            "credit_125scratch": 3.5,  # rare verbatim credit
            "pac_man_red": 2.0,      # rare attribution
            "atariage": 1.5,
            "taito": 1.5,
            "extend": 1.0,
            "hurry_up": 1.0,
            "help": 0.5,
            "round": 1.0,            # "ROUND" not "SOUND"
            "green_dragon_bub": 1.0,  # green dragon = player char
            "ghost_enemies": 1.5,    # ghost enemies (verified 2026-06-21)
        }
        for k, w in weights.items():
            max_score += w
            if probes.get(k):
                score += w
        # Food items (each worth 0.3, max 1.5 total for 5+ items)
        item_hits = sum(1 for k in ["item_watermelon", "item_banana", "item_cake",
                                    "item_ice_cream", "item_apple", "item_mushroom"]
                        if probes.get(k))
        max_score += 1.5
        score += min(item_hits * 0.3, 1.5)
        if probes.get("hallucinated_d2sereno"):
            score -= 3.0
            notes.append(f"hallucinated: {probes['hallucinated_d2sereno']}")

    elif image_name == "23-animation.jpg":
        weights = {
            "grid": 2.0,             # 2x3 / 6 frames
            "blue_cat": 2.0,
            "run_cycle": 3.5,        # key insight — most models get this wrong
            "stride": 1.0,
            "loop": 1.0,
            "frame_numbers": 0.5,
            "flat_shadow": 0.5,
        }
        for k, w in weights.items():
            max_score += w
            if probes.get(k):
                score += w

    elif image_name == "24-lymphatic-system-handdrawn.jpg":
        weights = {"title": 1.5}
        for k, w in weights.items():
            max_score += w
            if probes.get(k):
                score += w
        # 6 organs (0.5 each)
        for organ in ["tonsils", "thymus", "spleen", "vessels", "nodes", "marrow"]:
            max_score += 0.5
            if probes.get(f"organ_{organ}") or probes.get(organ):
                score += 0.5
        # Color coding (0.3 each = 1.2 total)
        for color_key in ["color_tonsils_pink", "color_thymus_orange",
                          "color_spleen_purple", "color_marrow_yellow"]:
            max_score += 0.3
            if probes.get(color_key):
                score += 0.3
        # Functions (0.5 each = 1.5 total)
        for func in ["func_infection", "func_fluid", "func_filter"]:
            max_score += 0.5
            if probes.get(func):
                score += 0.5
        # Style
        max_score += 0.5
        if probes.get("lined_paper"):
            score += 0.5

    elif image_name == "25-mri-brain-parkinson.jpeg":
        weights = {
            "mri": 1.5,
            "grid": 1.5,             # 2x3 / six slices
        }
        for k, w in weights.items():
            max_score += w
            if probes.get(k):
                score += w
        # Three planes (0.5 each = 1.5 total)
        for plane in ["coronal", "sagittal", "axial"]:
            max_score += 0.5
            if probes.get(f"plane_{plane}"):
                score += 0.5
        # Markers
        for marker, w in [("marker_r", 1.0), ("marker_al", 2.0), ("marker_mf", 1.0)]:
            max_score += w
            if probes.get(marker):
                score += w
        # Scale
        max_score += 1.0
        if probes.get("scale_5cm"):
            score += 1.0
        # Hallucination penalty
        if probes.get("hallucinated_lesions"):
            score -= 2.0
            notes.append(f"hallucinated lesions: {probes['hallucinated_lesions']}")

    elif image_name == "26-graph-ocean-acidification-hawaii.jpg":
        weights = {"title": 1.5}
        for k, w in weights.items():
            max_score += w
            if probes.get(k):
                score += w
        # Three series
        for series, w in [("series_atmos_co2", 1.0), ("series_ocean_pco2", 1.0),
                          ("series_ph", 1.0)]:
            max_score += w
            if probes.get(series):
                score += w
        # Date range
        for yr in ["year_1958", "year_2018"]:
            max_score += 0.5
            if probes.get(yr):
                score += 0.5
        # Stations (rare OCR)
        for station, w in [("mauna_loa", 1.5), ("aloha", 1.5), ("hawaii", 1.0)]:
            max_score += w
            if probes.get(station):
                score += w

    elif image_name == "27-graph-carbonate-based-marine-life-survival-against-ph.jpeg":
        weights = {"title": 2.0}
        for k, w in weights.items():
            max_score += w
            if probes.get(k):
                score += w
        # Three series
        for series, w in [("plankton", 1.0), ("dinoflagellate", 1.0), ("ph_series", 1.0)]:
            max_score += w
            if probes.get(series):
                score += w
        # X-axis (common error: 1950 vs 1940)
        max_score += 1.5
        if probes.get("year_1940"):
            score += 1.5
        max_score += 0.5
        if probes.get("year_2100"):
            score += 0.5
        # Annotations
        for ann, w in [("tipping_point", 1.0), ("current_position", 1.0),
                       ("domoic_acid", 2.0), ("starvation", 1.0)]:
            max_score += w
            if probes.get(ann):
                score += w
        if probes.get("hallucinated_atomic"):
            score -= 1.5
            notes.append("hallucinated 'atomic acid'")

    elif image_name == "28-xray-ribfracture-lowerright.jpg":
        weights = {
            "modality": 1.5,
            "view_pa": 1.5,
            "rib": 1.0,
        }
        for k, w in weights.items():
            max_score += w
            if probes.get(k):
                score += w
        # KEY FINDING: fracture (heavy weight)
        max_score += 4.0
        if probes.get("fracture"):
            score += 4.0
        # Specifics
        for k, w in [("rib_10th", 1.5), ("posterior", 1.0), ("location_left", 1.0)]:
            max_score += w
            if probes.get(k):
                score += w
        # Negative findings (worth less)
        for k in ["no_pneumothorax", "no_displacement"]:
            max_score += 0.5
            if probes.get(k):
                score += 0.5

    elif image_name == "29-painting-hundertwasser.jpeg":
        weights = {
            "artist": 2.5,           # identifying the artist
            "year_1965": 1.5,
            "yellow": 0.5,
            "onion_dome": 1.0,
            "blue_teardrop": 1.5,
        }
        for k, w in weights.items():
            max_score += w
            if probes.get(k):
                score += w
        # Buildings
        max_score += 1.0
        if probes.get("buildings_3"):
            score += 1.0
        max_score += 1.0
        if probes.get("red_tower"):
            score += 1.0
        # Face (signature motif)
        max_score += 1.5
        if probes.get("embedded_face"):
            score += 1.5
        max_score += 1.0
        if probes.get("red_cheeks"):
            score += 1.0
        if probes.get("hallucinated_water"):
            score -= 1.0
            notes.append("hallucinated water/sea in dark ground")

    elif image_name == "30-where-is-waldo.webp":
        weights = {
            "waldo_named": 3.0,      # genre identification
            "beach": 1.5,
            "crowd": 1.0,
            "beach_hut": 1.5,        # distinctive
            "striped": 1.0,
            "blue_white_stripes": 1.0,
            "red_white_stripes": 1.0,
            "boat": 1.0,
            "umbrella": 0.5,
            "horses": 2.0,           # owner-verified 2026-06-20
            "hovercraft": 1.5,       # verified 2026-06-21
            "ice_cream_cart": 1.0,   # verified 2026-06-21
        }
        for k, w in weights.items():
            max_score += w
            if probes.get(k):
                score += w
        # Bonus items (verified 2026-06-21) — each worth 0.3, max 1.5
        bonus_items = ["beach_chairs", "tent", "cooler", "balloon", "volleyball",
                       "surfboard", "human_pyramid", "punch_judy"]
        bonus_hits = sum(1 for k in bonus_items if probes.get(k))
        max_score += 1.5
        score += min(bonus_hits * 0.3, 1.5)
        # Bonus for actually claiming a Waldo location (judge verifies correctness)
        if probes.get("claims_waldo_location"):
            score += 1.0
        max_score += 1.0
        # Hallucinations (verified wrong)
        if probes.get("hallucinated_woof"):
            score -= 1.5
            notes.append("hallucinated Woof (owner could not verify)")
        if probes.get("hallucinated_dick_bruna"):
            score -= 2.0
            notes.append("hallucinated artist Dick Bruna (real: Handford)")
        if probes.get("hallucinated_pier"):
            score -= 1.0
            notes.append("hallucinated pier (real: hovercraft)")
        if probes.get("hallucinated_carriage"):
            score -= 1.0
            notes.append("hallucinated carriage")

    # Scale to 0-10
    if max_score > 0:
        normalized = max(0.0, min(10.0, (score / max_score) * 10.0))
    else:
        normalized = 5.0

    return normalized, "; ".join(notes) if notes else ""


# ============================================================================
# Layer 3: LLM-judge orchestration (separate script calls subagents)
# ============================================================================

def write_judge_input(image_name, ground_truth_text, responses):
    """Write a JSON file with all data the judge needs for one image."""
    out = JUDGE_DIR / f"input_{image_name.replace('/', '_')}.json"
    payload = {
        "image": image_name,
        "image_path": str(Path("test-images") / image_name),
        "ground_truth": ground_truth_text,
        "responses": responses,  # list of {model, content, failure_mode, deterministic_score}
    }
    out.write_text(json.dumps(payload, indent=2, ensure_ascii=False))
    return out


# ============================================================================
# Main
# ============================================================================

def load_results():
    """Load all benchmark results, return {image: {model: record}}."""
    results = defaultdict(dict)
    with open(RAW) as f:
        for line in f:
            r = json.loads(line)
            if r.get("type") == "result" and r.get("ok"):
                results[r["image"]][r["model"]] = r
    return results


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--mode", choices=["deterministic", "with-judge", "prepare-judge-input"],
                        default="deterministic",
                        help="What to compute")
    args = parser.parse_args()

    results = load_results()
    models = sorted(set(m for imgs in results.values() for m in imgs))
    images = sorted(results.keys())

    short = {'qwen3-vl-4b': 'Q3VL-4B', 'qwen3-vl-8b': 'Q3VL-8B', 'gemma4-12b': 'G4-12B',
        'gemma4-e4b': 'G4-E4B', 'gemma4-26b-a4b': 'G4-26B', 'gemma4-31b': 'G4-31B',
        'qwen3.6-27b': 'Q3.6-27B', 'glm-4.6v-flash-9b': 'GLM-9B',
        'qwen3.6-35b-a3b': 'Q3.6-35B', 'qwen3.5-4b': 'Q3.5-4B', 'qwen3.5-9b': 'Q3.5-9B'}

    # Compute deterministic layer
    print("=" * 120)
    print("VLM SCORECARD v4 — deterministic layer (failure modes + factual probes)")
    print("=" * 120)

    # Failure mode summary
    print("\n--- Failure mode breakdown ---")
    fm_counts = defaultdict(lambda: defaultdict(int))
    for img, model_results in results.items():
        for m, r in model_results.items():
            fm = detect_failure_mode(r)
            fm_counts[fm][m] += 1
    fms = ["normal", "truncated", "empty", "repetition_loop"]
    header = f"{'mode':<20}"
    for m in models:
        header += f" {short.get(m, m):>8}"
    print(header)
    for fm in fms:
        row = f"{fm:<20}"
        for m in models:
            row += f" {fm_counts[fm][m]:>8}"
        print(row)

    # Per-image scores
    print("\n--- Per-image deterministic scores (0-10) ---")
    header = f"{'image':<50}"
    for m in models:
        header += f" {short.get(m, m):>8}"
    print(header)
    print("-" * (50 + 9 * len(models)))

    scores = defaultdict(dict)  # image -> model -> score
    notes = defaultdict(dict)
    for img in images:
        row = f"  {img[:48]:<50}"
        for m in models:
            r = results.get(img, {}).get(m)
            if not r:
                row += f" {'—':>8}"
                continue
            fm = detect_failure_mode(r)
            probes = probes_for_image(img, r.get("content", ""))
            s, note = deterministic_score(img, probes, fm)
            scores[img][m] = s
            notes[img][m] = note
            row += f" {s:>8.1f}"
        print(row)

    # Compute totals — for now, equal weight per image (10 pts each)
    # (signal-based weights computed after we see the spread)
    print("-" * (50 + 9 * len(models)))
    totals = defaultdict(float)
    counts = defaultdict(int)
    for img in images:
        for m, s in scores[img].items():
            totals[m] += s
            counts[m] += 1

    row = f"  {'TOTAL (raw)':<50}"
    for m in models:
        if counts[m] == len(images):
            row += f" {totals[m]:>8.1f}"
        else:
            row += f" {'partial':>8}"
    print(row)
    row = f"  {'AVG (/10)':<50}"
    for m in models:
        if counts[m] > 0:
            row += f" {totals[m]/counts[m]:>8.2f}"
        else:
            row += f" {'—':>8}"
    print(row)

    # Spread
    avgs = [(m, totals[m] / counts[m]) for m in models if counts[m] > 0]
    if avgs:
        avgs.sort(key=lambda x: -x[1])
        print(f"\n  Spread: {avgs[0][1]:.2f} (top) − {avgs[-1][1]:.2f} (bottom) = {avgs[0][1] - avgs[-1][1]:.2f}")
        print(f"\n  Ranking:")
        medals = ['🥇', '🥈', '🥉']
        for i, (m, avg) in enumerate(avgs):
            medal = medals[i] if i < 3 else f"  {i+1}."
            print(f"    {medal} {short.get(m, m):<12} {avg:.2f}/10 ({counts[m]}/{len(images)})")

    if args.mode == "prepare-judge-input":
        # Write per-image judge input files
        print(f"\n--- Writing judge input files to {JUDGE_DIR} ---")
        gt_text = GROUND_TRUTH.read_text()
        # Split by image header
        gt_per_image = {}
        current = None
        buf = []
        for line in gt_text.splitlines():
            m = re.match(r'^## (\S+\.png|\S+\.jpg|\S+\.jpeg)', line)
            if m:
                if current:
                    gt_per_image[current] = "\n".join(buf).strip()
                current = m.group(1)
                buf = []
            elif current:
                buf.append(line)
        if current:
            gt_per_image[current] = "\n".join(buf).strip()

        for img in images:
            rs = []
            for m in models:
                r = results.get(img, {}).get(m)
                if not r:
                    continue
                fm = detect_failure_mode(r)
                probes = probes_for_image(img, r.get("content", ""))
                s, note = deterministic_score(img, probes, fm)
                rs.append({
                    "model": m,
                    "content": r.get("content", "")[:4000],  # truncate to fit context
                    "failure_mode": fm,
                    "deterministic_score": s,
                    "notes": note,
                })
            gt = gt_per_image.get(img, "")
            out = write_judge_input(img, gt, rs)
            print(f"  {img}: {len(rs)} responses, gt={len(gt)} chars → {out.name}")


if __name__ == "__main__":
    main()
