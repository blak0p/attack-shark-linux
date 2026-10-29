Translate the app UI into English and Spanish with a language selector.
Branch: feat/i18n-en-es (from feat/profile-process-watcher HEAD 8bdc4d4). Frontend only; Go backend/tray strings are out of scope for now.
Delivery forecast: several work-unit commits of ~400 lines; adds i18next + react-i18next (dependency, human-visible in package.json).

## Specs
S1. "quiero traducir la app a varios idiomas"
S2. "ingles y español que son los mas usados"
S3. English is the base and fallback language; Spanish is the second locale; default follows the system language, manual override persists.
S4. Existing tests keep passing with English as the default test language.
S5. English UI text stays byte-identical to today (existing tests pin it); never change English wording, plurals, or punctuation while migrating. Pin any oddity instead of fixing it.
S6. Conventions: components call `const { t } = useTranslation()` (from "react-i18next") before any early return; module-level helpers use `i18n.t` (import { i18n } from the i18n module). Keys live in frontend/src/i18n/locales/en.json and es.json with identical key sets (plural suffixes _one/_other, plus _many for Spanish, are the only allowed difference). Do not translate backend/user data (profile names, game names, executable names, codes). Logic that compares against English strings keeps comparing the English source; only displayed output is translated. Spanish uses Rioplatense voseo for instructions to the user (e.g. "Elegí", "Reintentá").
S7. Each migrated area gets a Spanish rendering test (see frontend/src/components/Translated.test.tsx for the pattern: setLanguage("es") in beforeEach, setLanguage("en") in afterEach).
S8. Hook- and validator-generated text (frontend/src/hooks/*.ts, frontend/src/macros/device-admission.ts, macro-file.ts) is produced through `i18n.t` at the moment it is set. Code that compares against those messages (WorkspaceFeedback, MacroManagerPanel) compares against the same `i18n.t(key)` result, never a separate hardcoded English literal, so the logic is unchanged in any language. Test files for these modules keep passing unchanged in English. Go backend strings (profile Notice, AppError/PolicyError, tray menu) are out of scope and stay as the backend sends them.

## Tasks
T1 | S1,S2,S3 | parent | done | i18n infrastructure (i18next init, en/es JSON, language detection + persistence, selector in TopBar), with tests. Tests: 660 pass, build ok. Commit: 310304f.
T2 | S1,S2,S4 | parent | done | Migrate App, workspace, feedback, updates components to t(). Tests: 663 pass, build ok (incl. Spanish rendering test). Commit: d1779c8.
T3 | S1,S2,S4 | parent | done | Migrate panels (DPI, polling, lighting, power, sleep, reset, features, device status). Tests: 665 pass, build ok. Commit: 0092d26.
T4 | S1,S2,S4,S5,S6,S7 | parent (remap, macros) + worker (profiles, games) | done | Migrate ButtonRemap (389bdca), MacroManager (65d1202), Profiles and Games panels (worker mvtowprx-1-l5fz under the runtime context backstop; parent read the full diff, reran npm test 670 pass and build, fixed the untranslated duplicate-profile default name). Risk: none (UI strings only; a message already on screen keeps its language until the next action). Commit: 09c49f9.
T6 | S1,S2,S5,S6,S7,S8 | worker mvtp12af-2-wckk (runtime context backstop) | done | Translate hook-generated text (notices in useDesktopWorkspace, feedbackFor fallbacks). Logic compares English notice strings today (WorkspaceFeedback), so switch those to codes first. Parent read the consumer diffs, reran npm test 673 pass + build, and checked mechanically that every removed English literal still exists in en.json. Risk: none. Commit: 3e204aa.
T5 | S1,S2 | parent | done | Docs (README language note), full check, es completeness test. README feature bullet, CONTRIBUTING translation guide. make check exit 0 (673 frontend tests, Go tests, Wails build). Browser e2e 42 pass. Visual check found a real regression (selector in the top bar overlapped the profile controls at 640px; e2e profiles layout test failed): moved the selector to the sidebar footer, verified by screenshots at 1280/640/400 and Spanish at 1280. Risk: none. Commit: 6b36a8e.

## Log
L1. User request verbatim: "quiero traducir la app a varios idiomas"
L2. User: "ingles y español que son los mas usados"
L3. Slice boundary: this feature starts at 8bdc4d4 (feat/profile-process-watcher). The 5706-line count vs origin/main is that branch's earlier commits, not this feature. T1 is its first slice (~250 lines).
L4. Finding: e2e was already broken on the base branch (GamesPanel calls GetAutostart since 324dd63, but e2e/entry.tsx never mocked it; 41 "Unexpected mock operation" errors, 1 test passed). Fixed in its own commit so the browser check is usable. Out of scope and left as is: Go backend strings (profile Notice, AppError/PolicyError, tray menu) and locale number formatting (1,000 Hz, 0.5 minutes).
