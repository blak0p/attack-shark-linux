Translate the app UI into English and Spanish with a language selector.
Branch: feat/i18n-en-es (from feat/profile-process-watcher HEAD 8bdc4d4). Frontend only; Go backend/tray strings are out of scope for now.
Delivery forecast: several work-unit commits of ~400 lines; adds i18next + react-i18next (dependency, human-visible in package.json).

## Specs
S1. "quiero traducir la app a varios idiomas"
S2. "ingles y español que son los mas usados"
S3. English is the base and fallback language; Spanish is the second locale; default follows the system language, manual override persists.
S4. Existing tests keep passing with English as the default test language.

## Tasks
T1 | S1,S2,S3 | parent | done | i18n infrastructure (i18next init, en/es JSON, language detection + persistence, selector in TopBar), with tests. Tests: 660 pass, build ok. Commit: 310304f.
T2 | S1,S2,S4 | parent | pending | Migrate App, workspace, feedback, updates components to t().
T3 | S1,S2,S4 | parent | pending | Migrate panels (DPI, polling, lighting, power, sleep, reset, features, device status).
T4 | S1,S2,S4 | parent | pending | Migrate ButtonRemap, MacroManager, Profiles, Games panels.
T5 | S1,S2 | parent | pending | Docs (README language note), full check, es completeness test.

## Log
L1. User request verbatim: "quiero traducir la app a varios idiomas"
L2. User: "ingles y español que son los mas usados"
L3. Slice boundary: this feature starts at 8bdc4d4 (feat/profile-process-watcher). The 5706-line count vs origin/main is that branch's earlier commits, not this feature. T1 is its first slice (~250 lines).
