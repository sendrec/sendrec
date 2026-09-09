# 99tools Record – Änderungen an der Web-Oberfläche

Basis: SendRec v1.90.4

Umgesetzt:
- Branding auf „99tools Record“ umgestellt
- 99tools-Logo in Header/Login eingebunden
- Favicons und Apple-Touch-Icon aus dem 99tools-Logo neu erzeugt
- Browser-Titel und Dokumentensprache auf Deutsch gesetzt
- Farbsystem auf 99tools angepasst:
  - Navy #0F172A
  - Raspberry #E6467A
  - Raspberry Hover #D63869
  - Off-White #F8FAFC
  - Slate #64748B
- sichtbare Benutzeroberfläche weitgehend vollständig deutsch lokalisiert
- Transkriptionssprachen mit deutschen Bezeichnungen versehen
- Open-Source-/AGPL-Hinweis in den Einstellungen ergänzt
- separate Open-Source-Hinweise in OPEN_SOURCE_99TOOLS.md ergänzt

Hinweis zur AGPL:
Vor einer externen/kommerziellen Bereitstellung muss der vollständige Corresponding Source der
modifizierten laufenden Version kostenfrei zugänglich gemacht werden. Der konkrete 99tools-Source-
Repository-Link ist noch einzutragen, sobald das Repository veröffentlicht ist.

## Mehrsprachige Oberfläche (Basis)

- eigenes leichtgewichtiges i18n-System ohne zusätzliche npm-Abhängigkeit
- Deutsch und Englisch als erste UI-Sprachen
- automatische Erkennung der Browsersprache beim ersten Besuch
- Speicherung der gewählten UI-Sprache in `localStorage` (`99tools-ui-language`)
- Sprachumschalter in Navigation, Login/Registrierung und Profil-Einstellungen
- `document.documentElement.lang` folgt der gewählten UI-Sprache
- Oberflächensprache und Transkriptionssprache bleiben getrennte Einstellungen
- Architektur über `src/i18n/translations.ts` für weitere Sprachen vorbereitet
- Navigation, Arbeitsbereich-Umschalter, Auth-Basis, Profil/Darstellung/Sprache und zentrale Einstellungen auf Übersetzungsschlüssel umgestellt

Hinweis: Die Migration aller Spezialseiten auf Übersetzungsschlüssel erfolgt schrittweise; Deutsch bleibt dabei vollständiger Fallback.

## i18n-Komplettierung DE/EN

- DE/EN-Fallback-Brücke für noch nicht auf `t(...)` migrierte Alt-Komponenten ergänzt
- exakte bekannte UI-Texte und relevante Attribute (`placeholder`, `title`, `aria-label`) werden sprachabhängig übersetzt
- Nutzerinhalte wie Videotitel, Kommentare oder Freitexte bleiben unangetastet
- Analytics-Zeiträume: Deutsch `7 T / 30 T / 90 T`, Englisch `7d / 30d / 90d`
- Settings, Library, Analytics, Video-Details, Playlists, Org-Settings und Dialoge umfassend DE/EN abgedeckt
- neue UI sollte weiterhin direkt `t("...")` verwenden; die Brücke ist bewusst eine Kompatibilitätsschicht

## AGPL / Corresponding Source

Die Einstellungen enthalten nun einen Button „Quellcode dieser Version / Source code for this version“.
Er erwartet die Datei unter:

`/source/99tools-record-source-v1.90.4.tar.gz`

Diese Datei muss vor dem produktiven Build aus dem vollständigen 99tools-Record-Quellbaum erzeugt und nach
`web/public/source/99tools-record-source-v1.90.4.tar.gz` kopiert werden. Sie muss den vollständigen Corresponding
Source der tatsächlich laufenden Version enthalten, nicht nur das Web-Frontend.
