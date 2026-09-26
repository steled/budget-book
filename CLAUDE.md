# CLAUDE.md — Arbeitsregeln für dieses Repository

## Pflichtschritte bei jeder Feature-Implementierung

Nach **jeder** Änderung an Go-Code, Templates oder statischen Assets sind folgende Schritte **automatisch und ohne gesonderte Aufforderung** durchzuführen:

### 1. Tests ausführen

```bash
go test ./...
```

Alle Tests müssen grün sein, bevor die Arbeit als abgeschlossen gilt. Schlägt ein Test fehl, ist zuerst die Ursache zu beheben — kein Überspringen.

### 2. Security-Analyse

Nach jeder Änderung, die neue Endpunkte, Authentifizierung, Cookie-Handling, Template-Rendering, Datenbankzugriffe oder externe Abhängigkeiten berührt, ist eine Security-Review mit `/security-review` durchzuführen.

Konkrete Auslöser (nicht abschließend):
- Neue HTTP-Handler oder Middleware
- Änderungen an `internal/auth/`
- Neue oder geänderte SQL-Queries
- Änderungen am Scheduler (`internal/scheduler/`), der ungefragt Buchungen anlegt
- Neue JavaScript-Dateien oder Template-Änderungen
- Neue Umgebungsvariablen mit sicherheitsrelevantem Inhalt
- Neue externe Abhängigkeiten (Go-Module, JS-Bibliotheken)

### 3. Design-Review (bei Frontend-Änderungen)

Nach jeder Änderung an CSS, Templates oder JavaScript ist folgende Checkliste zu prüfen:

- [ ] **Touch-Targets**: Alle Buttons/Links ≥ 2.75rem (44px) Mindestgröße auf Mobile?
- [ ] **Focus-Styles**: `focus-visible`-Indikator auf allen interaktiven Elementen vorhanden?
- [ ] **Kontrast**: Text gegen Hintergrund mind. WCAG AA (4.5:1) — auch im Dark Mode?
- [ ] **Keine Inline-Styles**: Ausschließlich CSS-Klassen, kein `style="…"` in Templates (per-Kategorie-Farben werden zur Laufzeit per JS auf das DOM-Element gesetzt, nicht als Attribut im Markup)?
- [ ] **Schriftgrößen**: Mindestens 0.875rem (14px) für UI-Text?
- [ ] **Icons**: SVG bevorzugen gegenüber Emojis für interaktive Elemente — außer bei bereits etablierten Emoji-Icons (z. B. Theme-Toggle 🌙/☀️, Konto-/Kategorie-Icons, Wiederholungs-Icon ↻), deren Fortführung Konsistenz mit dem Bestand schafft?
- [ ] **Dark Mode**: Neue Komponenten in beiden Themes geprüft?
- [ ] **Reduzierte Bewegung**: Animationen respektieren `prefers-reduced-motion`?
- [ ] **Neue CSS-Utilities**: Als Klassen in `style.css` angelegt, nicht als One-off-Inline-Styles?

Konkrete Auslöser (nicht abschließend):
- Neue oder geänderte CSS-Klassen/Variablen
- Neue interaktive Elemente (Buttons, Inputs, Links)
- Änderungen am Layout oder der Typografie
- Neue Template-Komponenten

### 4. Visuelle/interaktive Tests (bei Frontend-Änderungen)

Nach jeder Änderung an Templates, CSS oder JavaScript sind die neuen bzw. geänderten Interaktionen tatsächlich auszuführen — nicht nur Code zu lesen, `go test` laufen zu lassen oder API-Endpunkte per `curl` zu prüfen. Die Aufgabe gilt erst als abgeschlossen, wenn diese Szenarien tatsächlich durchlaufen wurden, nicht schon nach reiner Code-Fertigstellung.

- **Bevorzugt: automatisiert im Browser** mit Playwright (Python). In dieser Dev-Umgebung ist es installiert unter `~/.venvs/playwright` (Chromium headless, Systembibliotheken via `playwright install-deps`). Ablauf:
  1. Dev-Server mit **eigener Wegwerf-Datenbank** im Scratchpad starten, nie gegen echte Daten: `APP_PASSWORD=test APP_SESSION_SECRET=$(openssl rand -hex 32) DATABASE_PATH=<scratchpad>/test.db APP_ADDR=127.0.0.1:8098 go run .` (der Scheduler läuft beim Start mit — für Tests mit wiederkehrenden Buchungen die Vorlagen vorher per API anlegen und den Server danach neu starten).
  2. Testdaten über die API anlegen, dann ein Skript mit `~/.venvs/playwright/bin/python` ausführen, das die aus dem Diff abgeleiteten Szenarien durchklickt und je Schritt das erwartete Ergebnis prüft (Login über `#username`/`#password`, Tabs über `.tab-btn[data-tab=…]`).
  3. Immer mitprüfen: Browser-Konsole (`console`/`pageerror`) ohne Fehler — auch CSP-Verstöße, die sonst unbemerkt bleiben —, Screenshots in Hell **und** Dunkel sowie bei 375px Breite (selbst ansehen, Layoutfehler finden Asserts nicht), gerenderter Kontrast ≥ 4.5:1, Touch-Targets ≥ 44px, kein horizontales Scrollen, Tastatur-Fokus.
- **Steht keine Automatisierung zur Verfügung** (z. B. in einer anderen Umgebung ohne `~/.venvs/playwright`): Dev-Server starten (`APP_PASSWORD=... APP_SESSION_SECRET=$(openssl rand -hex 32) go run .`) und dem Nutzer eine **konkrete, auf die gerade hinzugefügte/geänderte Funktion zugeschnittene** Liste von Testszenarien vorgeben — Schritt für Schritt, mit erwartetem Ergebnis je Punkt. Keine generische/wiederverwendete Standard-Checkliste — die Szenarien müssen jedes Mal neu aus dem tatsächlichen Diff abgeleitet werden (z. B. bei einer Änderung am Wiederholungs-Scheduler: konkret einen Rückstand simulieren, App neu starten, prüfen wie viele Buchungen mit `source: recurring` nachgeholt wurden).
- Rückmeldungen aus diesen Tests (Bugs, unerwartetes Verhalten) sind vor Abschluss der Aufgabe zu beheben, nicht nur zu notieren.

Konkrete Auslöser: identisch zu Abschnitt 3 (neue/geänderte CSS-Klassen, neue interaktive Elemente, Layout-/Typografie-Änderungen, neue Template-Komponenten).

### 5. Dokumentation aktualisieren

`README.md` ist zu aktualisieren, wenn sich folgendes ändert:

| Änderung | README-Abschnitt |
|----------|-----------------|
| Neue/geänderte Umgebungsvariable | **Environment Variables** |
| Neuer/geänderter API-Endpunkt | **API Reference** |
| Neuer GitHub-Actions-Workflow | **CI/CD** |
| Neue Feature-Funktionalität | **Features** |
| Neue externe Abhängigkeit | **Tech Stack** |

---

## Projektkonventionen

### Commit-Format

Dieses Projekt verwendet [Conventional Commits](https://www.conventionalcommits.org/). Jeder Commit-Message-Prefix bestimmt das automatische Versioning:

| Prefix | Auswirkung |
|--------|-----------|
| `feat:` | Minor-Release |
| `fix:`, `perf:` | Patch-Release |
| `feat!:` / `BREAKING CHANGE` | Major-Release |
| `chore:`, `docs:`, `ci:`, `refactor:` | Kein Release |

### Code-Stil

- **Go**: Kein unnötiger Kommentar-Overhead. Kommentare nur wenn das *Warum* nicht offensichtlich ist.
- **SQL**: Ausschließlich parametrisierte Queries (`?`-Platzhalter) — niemals String-Konkatenation.
- **JavaScript**: Kein `eval()`, kein `innerHTML` mit nicht-escapten Benutzerdaten. Für Benutzerdaten `textContent` oder die `esc()`-Hilfsfunktion verwenden.
- **Templates**: Go's `html/template`-Paket wird verwendet — kein `template.HTML`-Cast ohne explizite Begründung.
- **Geldbeträge**: Ausschließlich als Integer-Cent (`amountCents`, `int64`) speichern und über die API transportieren — nie als `float64`, um Rundungsfehler zu vermeiden. Das Vorzeichen (Einnahme/Ausgabe) ergibt sich aus `category.type`, nicht aus dem Betrag selbst.

### Sicherheitsregeln (nicht verhandelbar)

- Session-Cookies: immer `HttpOnly: true`, `SameSite: Lax`, `Secure: a.secureCookies || r.TLS != nil`
- Passwörter: ausschließlich bcrypt (`bcrypt.DefaultCost` oder höher)
- Session-Signatur: HMAC-SHA256, konstanter Zeitvergleich (`hmac.Equal`)
- `APP_SESSION_SECRET` muss mindestens 32 Zeichen haben (wird beim Start geprüft)
- CSP ist `default-src 'self'` — keine CDN-Whitelist ohne Bundling + explizite Begründung
- Neue externe JS-Bibliotheken: lokal in `static/` bundeln, Version in `static/<lib>.version` festhalten (siehe Referenz-Repo `shopping-list`'s `update-sortable.yml`-Workflow als Vorlage, falls eine JS-Bibliothek nötig wird)

### Datenbankmigrationen

Schemaänderungen ausschließlich über die `migrate()`-Funktion in `internal/database/database.go`. Neue Tabellen per `CREATE TABLE IF NOT EXISTS`, Spaltenänderungen an bestehenden Tabellen per `columnExists()`-Guard vor `ALTER TABLE ADD COLUMN` (SQLite kennt kein `ADD COLUMN IF NOT EXISTS`). Keine destruktiven Migrationen ohne explizite Bestätigung durch den Nutzer.

### Wiederkehrende Buchungen (Scheduler)

- Läuft beim App-Start und danach alle 24h (`internal/scheduler`), kein externer Cronjob.
- Nachholen verpasster Termine: Schleife pro Vorlage, bis `next_due_date > heute`.
- Erzeugte Buchungen bekommen `source: "recurring"` (rein informativ) und bleiben danach ganz normal editierbar — ein Edit ändert nicht das Flag.

---

## Projektstruktur (Übersicht)

```
.
├── main.go                              # Einstiegspunkt, HTTP-Routing, Middleware, Scheduler-Start
├── internal/
│   ├── auth/auth.go                     # Session-Cookie, HMAC-Signatur, bcrypt
│   ├── database/                        # SQLite-Zugriff, Migrationen, alle SQL-Queries
│   │   ├── database.go                  #   Open/migrate, gemeinsame Fehler-Helfer
│   │   ├── accounts.go
│   │   ├── categories.go
│   │   ├── transactions.go
│   │   └── recurring.go                 #   inkl. ProcessDueRecurringTemplates (Catch-up)
│   ├── scheduler/scheduler.go           # täglicher Ticker, ruft ProcessDueRecurringTemplates
│   └── handlers/                        # HTTP-Handler (Login, Seiten, JSON-API)
│       ├── handlers.go                  #   Setup, Login/Logout, Rate-Limiter
│       ├── accounts.go / categories.go / transactions.go / recurring.go
│       └── overview.go / month.go
├── templates/
│   ├── base.html                        # Basis-Layout (Header, Footer, Theme-Script-Ref)
│   ├── index.html                       # App-Shell: Übersicht-/Monat-/Einstellungen-Tabs
│   └── login.html                       # Login-Formular
├── static/
│   ├── style.css
│   ├── app.js                           # Tabs, Übersicht, Monat, Einstellungen, Buchungs-Dialog
│   └── theme.js                         # Dark-Mode-Toggle
├── helm/budget-book/                    # Helm-Chart für Kubernetes
├── docker/Dockerfile                    # Multi-Stage-Build
└── .github/workflows/                   # CI/CD-Workflows
```

---

## Sicherheits-Checkliste für neue HTTP-Endpunkte

Bei jedem neuen Endpunkt prüfen:

- [ ] Authentifizierung: `h.auth.RequireAuth(...)` in `Register()` gewrappt?
- [ ] Input-Validierung: Alle Felder validiert, Beträge als Cent geparst, Referenzen (Konto/Kategorie) auf Existenz geprüft?
- [ ] SQL: Parametrisierte Query, kein String-Formatting?
- [ ] Response: Korrekte HTTP-Statuscodes (201, 204, 400, 401, 404, 409, 500)?
- [ ] Error-Handling: Kein Stack-Trace oder interne Details in der Response?
- [ ] Methode korrekt: GET für lesend, POST/PUT/DELETE für schreibend?
