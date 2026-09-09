import type { UiLanguage } from "./translations";

/**
 * Compatibility dictionary for UI strings that still live in legacy components.
 * New/changed UI should use t("...") directly. This bridge keeps DE/EN complete
 * while the remaining legacy components are migrated incrementally.
 */
const PAIRS: Array<[de: string, en: string]> = [
  // Common
  ["Speichern", "Save"], ["Wird gespeichert...", "Saving..."], ["Gespeichert", "Saved"],
  ["Speichern fehlgeschlagen", "Save failed"], ["Abbrechen", "Cancel"], ["Löschen", "Delete"],
  ["Kopieren", "Copy"], ["Kopiert!", "Copied!"], ["Zurück", "Back"], ["Ansehen", "View"],
  ["Bearbeiten", "Edit"], ["Entfernen", "Remove"], ["Neu erstellen", "Create new"],
  ["Wird erstellt...", "Creating..."], ["Wird geladen...", "Loading..."], ["Wird gesendet...", "Sending..."],
  ["Wird gelöscht...", "Deleting..."], ["Wird entfernt...", "Removing..."], ["Wird aktualisiert...", "Updating..."],
  ["Aktiv", "Active"], ["Aus", "Off"], ["An", "On"], ["Keine", "None"], ["Alle", "All"],
  ["Privat", "Private"], ["Öffentlich", "Public"], ["Status", "Status"], ["Bezeichnung", "Label"],

  // Analytics
  ["Analysen", "Analytics"], ["Video", "Video"], ["Dashboard", "Dashboard"], ["Export CSV", "Export CSV"],
  ["Analysedaten konnten nicht geladen werden.", "Analytics data could not be loaded."],
  ["Aufrufe gesamt", "Total views"], ["Eindeutige Zuschauer", "Unique viewers"],
  ["Eindeutige Aufrufe", "Unique views"], ["Ø / Tag", "Avg / day"], ["Videos gesamt", "Total videos"],
  ["Wiedergabezeit", "Watch time"], ["Ø Wiedergabe", "Avg completion"], ["Stärkster Tag", "Best day"],
  ["CTA-Klicks", "CTA clicks"], ["Top-Videos", "Top videos"], ["Top-Quellen", "Top sources"],
  ["Aufrufe", "Views"], ["unique", "unique"], ["completion", "completion"],
  ["Aufrufe im Zeitverlauf", "Views over time"], ["Zuschauerbindung", "Viewer retention"],
  ["Zuschaueraktivität", "Viewer activity"], ["Noch keine Aufrufe", "No views yet"],
  ["Noch keine Analysedaten", "No analytics yet"],
  ["Aufrufe erscheinen hier, sobald deine Videos angesehen werden.", "Views will appear here once your videos are watched."],
  ["Keine Aufrufe in diesem Zeitraum", "No views in this period"],
  ["Teile dein Video, damit Analysedaten erfasst werden können.", "Share your video to start collecting analytics."],
  ["Letzte 7 Tage", "Last 7 days"], ["Letzte 30 Tage", "Last 30 days"], ["Letzte 90 Tage", "Last 90 days"],
  ["Gesamter Zeitraum", "All time"], ["7 T", "7d"], ["30 T", "30d"], ["90 T", "90d"],
  ["Completion Funnel", "Completion funnel"], ["Browsers", "Browsers"], ["Devices", "Devices"],

  // Recording / recorder leftovers
  ["Vorschau einklappen", "Collapse preview"], ["Vorschau ausklappen", "Expand preview"],
  ["Klicken, um sofort zu starten", "Click to start immediately"], ["Fehler schließen", "Close error"],
  ["Zeichnen ausschalten", "Turn drawing off"], ["Zeichnen einschalten", "Turn drawing on"],
  ["Zeichenfarbe", "Drawing color"], ["Zeichnung löschen", "Clear drawing"],
  ["Aufnahme fortsetzen", "Resume recording"], ["Aufnahme pausieren", "Pause recording"],
  ["Aufnahme stoppen", "Stop recording"],

  // Library
  ["Bibliothek", "Library"], ["Alle Videos", "All videos"], ["Ohne Ordner", "Unfiled"], ["Ordner", "Folders"],
  ["Tags", "Tags"], ["Neuer Ordner", "New folder"], ["Neuer Tag", "New tag"],
  ["Videos durchsuchen...", "Search videos..."], ["Neue Aufnahme", "New recording"],
  ["Neueste zuerst", "Newest first"], ["Älteste zuerst", "Oldest first"], ["Meistgesehen", "Most viewed"],
  ["Titel A–Z", "Title A–Z"], ["Videos sortieren", "Sort videos"], ["Alle auswählen", "Select all"],
  ["Auswahl aufheben", "Clear selection"], ["Link kopieren", "Copy link"], ["Link kopiert", "Link copied"],
  ["Noch keine Aufrufe", "No views yet"], ["Läuft nie ab", "Never expires"],
  ["wird hochgeladen...", "uploading..."], ["Wird hochgeladen...", "Uploading..."],
  ["wird verarbeitet...", "processing..."], ["Wird verarbeitet...", "Processing..."],
  ["Aufnahme wird hochgeladen...", "Recording is uploading..."], ["Noch keine Aufnahmen vorhanden.", "No recordings yet."],
  ["Verschieben nach...", "Move to..."], ["Videos verschoben", "Videos moved"],
  ["Videos konnten nicht verschoben werden", "Videos could not be moved"],
  ["Videos konnten nicht gelöscht werden", "Videos could not be deleted"],
  ["Video verschoben", "Video moved"], ["Video angeheftet", "Video pinned"], ["Video nicht mehr angeheftet", "Video unpinned"],
  ["Angeheftet", "Pinned"], ["Anheften konnte nicht aktualisiert werden", "Could not update pin"],

  // Settings general/profile
  ["Aufnahme-Standards", "Recording defaults"],
  ["Lege deinen bevorzugten Aufnahmemodus und die Optionen fest.", "Set your preferred recording mode and options."],
  ["Standard-Aufnahmemodus", "Default recording mode"], ["Aufnahmemodus", "Recording mode"],
  ["Kamera", "Camera"], ["Bildschirm", "Screen"], ["Bildschirm + Kamera", "Screen + camera"],
  ["Countdown", "Countdown"], ["Systemaudio aufnehmen", "Record system audio"],
  ["Audio", "Audio"], ["Automatisch erkennen", "Detect automatically"],
  ["Datenaufbewahrung", "Data retention"], ["Automatisch löschen nach", "Automatically delete after"],
  ["30 Tagen", "30 days"], ["60 Tagen", "60 days"], ["90 Tagen", "90 days"], ["180 Tagen", "180 days"], ["365 Tagen", "365 days"],
  ["Integrationen", "Integrations"],
  ["Verbinde externe Dienste, um aus Videos Issues zu erstellen.", "Connect external services to create issues from videos."],
  ["Nicht verbunden", "Not connected"], ["Verbunden", "Connected"], ["Verbindung testen", "Test connection"],
  ["Verbindung fehlgeschlagen", "Connection failed"], ["Trennen", "Disconnect"], ["Trennen fehlgeschlagen", "Disconnect failed"],
  ["Repository", "Repository"], ["Projekt-Key", "Project key"], ["Persönlicher Zugriffstoken", "Personal access token"],

  // Notifications / webhooks
  ["E-Mail-Benachrichtigungen", "Email notifications"],
  ["Lege fest, wann du E-Mail-Benachrichtigungen zu Aufrufen und Kommentaren erhältst.", "Choose when to receive email notifications for views and comments."],
  ["Benachrichtigungen", "Notifications"], ["Nur Kommentare", "Comments only"], ["Nur Aufrufe", "Views only"],
  ["Aufrufe + Kommentare", "Views + comments"], ["Tägliche Zusammenfassung", "Daily digest"],
  ["Tägliche Zusammenfassung (Aufrufe + Kommentare)", "Daily digest (views + comments)"],
  ["Slack-Benachrichtigungen", "Slack notifications"],
  ["Sende Benachrichtigungen zu Videoaufrufen und Kommentaren an einen Slack-Kanal.", "Send notifications for video views and comments to a Slack channel."],
  ["Slack-Webhook-URL", "Slack webhook URL"], ["Testnachricht senden", "Send test message"],
  ["Testnachricht gesendet", "Test message sent"], ["Testnachricht konnte nicht gesendet werden", "Test message could not be sent"],
  ["So erhältst du eine Webhook-URL", "How to get a webhook URL"],
  ["Gehe zu", "Go to"], ["Klicke auf", "Click"], ["und wähle", "and choose"],
  ["Aktiviere Webhooks und klicke auf", "Enable Webhooks and click"],
  ["Wähle einen Kanal und kopiere die Webhook-URL", "Choose a channel and copy the webhook URL"],
  ["Webhooks", "Webhooks"],
  ["Empfange HTTP-POST-Benachrichtigungen zu Video-Ereignissen. Nutzbar mit n8n, Zapier oder eigenen Integrationen.", "Receive HTTP POST notifications for video events. Works with n8n, Zapier, or custom integrations."],
  ["Webhook-URL", "Webhook URL"], ["Signatur-Secret", "Signing secret"], ["Secret neu erzeugen", "Regenerate secret"],
  ["Wird neu erzeugt...", "Regenerating..."], ["Webhook speichern", "Save webhook"], ["Testereignis senden", "Send test event"],
  ["Testereignis gesendet", "Test event sent"], ["Test konnte nicht gesendet werden", "Test could not be sent"],
  ["Letzte Zustellungen", "Recent deliveries"], ["Nach Ereignis filtern...", "Filter by event..."],
  ["Erfolgreich", "Success"], ["Fehler", "Error"], ["Keine passenden Zustellungen", "No matching deliveries"],
  ["Nutzdaten", "Payload"], ["Antwort", "Response"], ["Unterstützte Ereignisse", "Supported events"],

  // Security / account
  ["Passwort ändern", "Change password"], ["Aktuelles Passwort", "Current password"], ["Neues Passwort", "New password"],
  ["Neues Passwort bestätigen", "Confirm new password"], ["Mindestens 8 Zeichen erforderlich", "At least 8 characters required"],
  ["API-Schlüssel", "API keys"], ["Schlüssel erstellen", "Create key"], ["Unbenannter Schlüssel", "Unnamed key"],
  ["Kopiere diesen Schlüssel jetzt – er wird später nicht erneut angezeigt", "Copy this key now — it will not be shown again"],
  ["Verknüpfte Konten", "Connected accounts"], ["Gefahrenbereich", "Danger zone"], ["Abmelden", "Sign out"],
  ["Melde dein Konto auf diesem Gerät ab.", "Sign out of your account on this device."], ["Konto löschen", "Delete account"],
  ["Lösche dein Konto und alle Daten dauerhaft.", "Permanently delete your account and all data."],
  ["Dies kann nicht rückgängig gemacht werden. Alle Videos und Daten werden dauerhaft gelöscht.", "This cannot be undone. All videos and data will be permanently deleted."],

  // Open source
  ["Open Source & Lizenzen", "Open Source & Licenses"],
  ["Der zu dieser Installation gehörende Quellcode wird gemäß AGPL-3.0 ohne zusätzliche Lizenzgebühr bereitgestellt.", "The corresponding source code for this installation is provided under AGPL-3.0 at no additional license fee."],
  ["Quellcode dieser Version", "Source code for this version"], ["SendRec-Originalprojekt", "Original SendRec project"],
  ["AGPL-3.0-Lizenz", "AGPL-3.0 License"],

  // Auth/password
  ["Passwort zurücksetzen", "Reset password"], ["Link zum Zurücksetzen senden", "Send reset link"],
  ["Prüfe dein E-Mail-Postfach", "Check your email"], ["Zurück zur Anmeldung", "Back to sign in"],
  ["Neuen Link zum Zurücksetzen anfordern", "Request a new reset link"], ["Ungültiger Link zum Zurücksetzen", "Invalid reset link"],
  ["Passwort aktualisiert", "Password updated"], ["Neues Passwort festlegen", "Set a new password"],

  // Playlist / organization
  ["Zurück zu Playlists", "Back to playlists"], ["Playlist löschen", "Delete playlist"], ["Playlist nicht gefunden", "Playlist not found"],
  ["Noch keine Videos in dieser Playlist", "No videos in this playlist yet"], ["Videos hinzufügen", "Add videos"],
  ["Öffentlicher Link", "Public link"], ["Passwort setzen", "Set password"], ["Passwort entfernen", "Remove password"],
  ["Ablauf festlegen", "Set expiry"], ["Ablauf entfernen", "Remove expiry"],
  ["Allgemein", "General"], ["Mitglieder", "Members"], ["Rolle", "Role"], ["Besitzer", "Owner"], ["Mitglied", "Member"], ["Betrachter", "Viewer"],
  ["Ausstehende Einladungen", "Pending invitations"], ["Widerrufen", "Revoke"], ["Single Sign-On", "Single Sign-On"],
  ["SSO für alle Mitglieder erzwingen", "Require SSO for all members"], ["SSO-Einstellungen speichern", "Save SSO settings"],
  ["SCIM-Bereitstellung", "SCIM provisioning"], ["SCIM-Token erzeugen", "Generate SCIM token"], ["SCIM-Token widerrufen", "Revoke SCIM token"],

  // Video detail / sharing / editing
  ["Zur Bibliothek", "Back to library"], ["Video nicht gefunden", "Video not found"], ["Video konnte nicht geladen werden", "Video could not be loaded"],
  ["Titel bearbeiten", "Edit title"], ["Titel aktualisiert", "Title updated"], ["Beschreibung hinzufügen", "Add description"],
  ["Beschreibung bearbeiten", "Edit description"], ["Beschreibung hinzufügen...", "Add description..."],
  ["Bearbeitung", "Editing"], ["Vorgeschlagener Titel", "Suggested title"], ["Trimmen", "Trim"],
  ["Stille", "Silence"], ["Stille entfernen", "Remove silence"], ["Organize", "Organize"],
  ["Playlists durchsuchen...", "Search playlists..."], ["Call-to-Action", "Call to action"],
  ["CTA bearbeiten", "Edit CTA"], ["CTA hinzufügen", "Add CTA"], ["CTA entfernen", "Remove CTA"],
  ["CTA gespeichert", "CTA saved"], ["CTA entfernt", "CTA removed"], ["CTA konnte nicht gespeichert werden", "CTA could not be saved"],
  ["Button-Text (z. B. Demo buchen)", "Button text (e.g. Book a demo)"],
  ["URL (e.g. https://example.com/demo)", "URL (e.g. https://example.com/demo)"],
  ["Freigabe-Einstellungen", "Sharing settings"], ["Freigabelink", "Share link"], ["Einbetten", "Embed"],
  ["Einbettungscode", "Embed code"], ["Passwort", "Password"], ["Ablauf", "Expiry"], ["Downloads", "Downloads"],
  ["E-Mail-Abfrage", "Email capture"], ["Kommentare", "Comments"], ["Kommentarmodus", "Comment mode"],
  ["Anonym", "Anonymous"], ["Name erforderlich", "Name required"], ["Name + E-Mail", "Name + email"],
  ["Vorschaubild", "Thumbnail"], ["Kontostandard", "Account default"], ["Jeder Aufruf", "Every view"],
  ["Tägliche Zusammenfassung", "Daily digest"], ["Branding", "Branding"], ["Anpassen", "Customize"],
  ["Vom Konto übernehmen", "Use account default"], ["Branding gespeichert", "Branding saved"],
  ["Branding konnte nicht gespeichert werden", "Branding could not be saved"],
  ["Passwort von diesem Video entfernen?", "Remove password from this video?"], ["Passwort gesetzt", "Password set"],
  ["Passwort entfernt", "Password removed"], ["Link verlängert", "Link extended"], ["Ablauf entfernen", "Remove expiry"],
  ["Vorschaubild aktualisiert", "Thumbnail updated"], ["Vorschaubild zurückgesetzt", "Thumbnail reset"],
  ["Vorschaubild zurücksetzen", "Reset thumbnail"], ["Transkript", "Transcript"],
  ["Transkript hochladen", "Upload transcript"], ["Transkript hochgeladen", "Transcript uploaded"],
  ["Transkript neu erstellen", "Regenerate transcript"], ["Transkription erneut versuchen", "Retry transcription"],
  ["Zusammenfassung", "Summary"], ["Zusammenfassung wird erstellt...", "Generating summary..."],
  ["Weitere Aktionen", "More actions"], ["Video löschen", "Delete video"],

  // Remaining legacy dialogs and status messages
  ["Füllwörter konnten nicht entfernt werden", "Filler words could not be removed"],
  ["Füllwörter entfernen", "Remove filler words"], ["Transkript wird geladen...", "Loading transcript..."],
  ["Transkript konnte nicht geladen werden.", "Transcript could not be loaded."],
  ["Im Transkript dieses Videos wurden keine Füllwörter erkannt.", "No filler words were detected in this video's transcript."],
  ["Trimmen fehlgeschlagen", "Trimming failed"], ["Wird getrimmt...", "Trimming..."], ["Video trimmen", "Trim video"],
  ["Stille Pausen konnten nicht entfernt werden", "Silent pauses could not be removed"],
  ["Stille Pausen entfernen", "Remove silent pauses"], ["Stille wird erkannt...", "Detecting silence..."],
  ["Stille konnte nicht erkannt werden.", "Silence could not be detected."],
  ["Video konnte nicht übertragen werden", "Video could not be transferred"], ["Video übertragen", "Transfer video"],
  ["Wird verschoben...", "Moving..."], ["Video verschieben", "Move video"],
  ["Keine weiteren Arbeitsbereiche verfügbar.", "No other workspaces available."],
  ["In die Zwischenablage kopieren", "Copy to clipboard"],
  ["Auf die Kamera konnte nicht zugegriffen werden. Bitte erlaube den Kamerazugriff und versuche es erneut.", "Could not access the camera. Please allow camera access and try again."],
  ["Die Aufnahme ist zu kurz. Bitte nimm mindestens 1 Sekunde auf.", "The recording is too short. Please record at least 1 second."],
  ["Bildschirmaufnahme fehlgeschlagen", "Screen recording failed"],
  ["Die Bildschirmaufnahme wurde blockiert oder ist fehlgeschlagen. Bitte erlaube die Bildschirmfreigabe und versuche es erneut.", "Screen recording was blocked or failed. Please allow screen sharing and try again."],
  ["Bitte lade die Seite neu und versuche es erneut.", "Please reload the page and try again."],

  // Playlists
  ["Playlist konnte nicht erstellt werden", "Playlist could not be created"],
  ["Diese Playlist löschen? Die Videos selbst werden nicht gelöscht.", "Delete this playlist? The videos themselves will not be deleted."],
  ["Neue Playlist", "New playlist"], ["Playlist-Titel", "Playlist title"], ["Erstellen", "Create"],
  ["Videos", "Videos"], ["Noch keine Playlists", "No playlists yet"],
  ["Erstelle eine Playlist, um Videos zu organisieren und gemeinsam zu teilen.", "Create a playlist to organize and share videos together."],
  ["Erste Playlist erstellen", "Create first playlist"], ["Videos konnten nicht hinzugefügt werden", "Videos could not be added"],
  ["Wird hinzugefügt...", "Adding..."], ["Keine verfügbaren Videos zum Hinzufügen", "No available videos to add"],
  ["Passwort für diese Playlist eingeben:", "Enter a password for this playlist:"],
  ["Passwort von dieser Playlist entfernen?", "Remove password from this playlist?"], ["Kein Passwort", "No password"],

  // Upload and record states
  ["Upload fehlgeschlagen", "Upload failed"], ["Video wird erstellt...", "Creating video..."],
  ["Video konnte nicht erstellt werden", "Video could not be created"], ["Kamera-Aufnahme wird hochgeladen...", "Uploading camera recording..."],
  ["Wird fertiggestellt...", "Finalizing..."], ["Bitte diese Seite nicht schließen", "Please do not close this page"],
  ["Aufnahme ist nicht verfügbar", "Recording is unavailable"], ["ein Video hoch", "upload a video"],
  ["Lösche nicht benötigte Aufnahmen oder warte bis zum nächsten Monat.", "Delete recordings you no longer need or wait until next month."],
  ["Dein Video ist fertig!", "Your video is ready!"], ["Video ansehen", "View video"], ["Weitere Aufnahme", "Record another"],
  ["Unterstützt werden nur MP4-, WebM- und MOV-Dateien", "Only MP4, WebM, and MOV files are supported"],
  ["Monatliches Video-Limit erreicht", "Monthly video limit reached"], ["Upload konnte nicht erstellt werden", "Upload could not be created"],
  ["Weitere hochladen", "Upload more"], ["Klicken oder weitere Dateien hier ablegen", "Click or drop more files here"],
  ["Ziehe deine Videos hierher", "Drop your videos here"],

  // Library remaining
  ["Wird heruntergeladen...", "Downloading..."], ["In Ordner verschieben...", "Move to folder..."], ["Kein Ordner", "No folder"],
  ["Diese Aufnahme löschen? Dies kann nicht rückgängig gemacht werden.", "Delete this recording? This cannot be undone."],
  ["Diesen Ordner löschen? Die Videos werden keinem Ordner mehr zugeordnet.", "Delete this folder? Its videos will become unfiled."],
  ["Diesen Tag löschen? Er wird von allen Videos entfernt.", "Delete this tag? It will be removed from all videos."],

  // Video detail remaining
  ["Issue konnte nicht erstellt werden", "Issue could not be created"], ["Video-Vorschaubild", "Video thumbnail"],
  ["Video lösen", "Unpin video"], ["Video anheften", "Pin video"], ["Issue erstellen", "Create issue"],
  ["Füllwörter werden entfernt...", "Removing filler words..."], ["Stille Pausen werden entfernt...", "Removing silent pauses..."],
  ["Verfügbar, sobald die Verarbeitung abgeschlossen ist", "Available once processing is complete"],
  ["Video-Branding", "Video branding"],
  ["Überschreibe das Konto-Branding für dieses Video. Leer lassen, um die Konto-Einstellungen zu übernehmen.", "Override account branding for this video. Leave blank to use the account settings."],
  ["Aufruf-Benachrichtigungen", "View notifications"],
  ["Transkript konnte nicht hochgeladen werden", "Transcript could not be uploaded"], ["Transkript zu kurz", "Transcript too short"],
  ["Transkription eingereiht", "Transcription queued"], ["Zusammenfassung eingereiht", "Summary queued"],

  // Branding settings
  ["Das Logo muss PNG oder SVG sein", "The logo must be PNG or SVG"], ["Das Logo darf maximal 512 KB groß sein", "The logo may be at most 512 KB"],
  ["Upload-URL konnte nicht erstellt werden", "Upload URL could not be created"], ["Logo konnte nicht hochgeladen werden", "Logo could not be uploaded"],
  ["Logo konnte nicht entfernt werden", "Logo could not be removed"], ["Logo hochladen (PNG oder SVG, max. 512 KB)", "Upload logo (PNG or SVG, max. 512 KB)"],
  ["Geteilt mit 99tools Record", "Shared with 99tools Record"], ["Kommentar veröffentlichen", "Post comment"],
  ["Branding speichern", "Save branding"], ["Passe das Erscheinungsbild deiner freigegebenen Videoseiten an.", "Customize the appearance of your shared video pages."],
  ["Standardlogo anzeigen", "Show default logo"], ["Vorschau", "Preview"],
  ["Wird in das <style>-Tag der Wiedergabeseite eingefügt. Max. 10 KB. Kein @import url() und keine schließenden style-Tags.", "Inserted into the watch page <style> tag. Max. 10 KB. No @import url() or closing style tags."],
  ["Auf Standard zurücksetzen", "Reset to default"],

  // Billing
  ["Abrechnung", "Billing"], ["Abonnement", "Subscription"], ["Auf Pro upgraden", "Upgrade to Pro"], ["Auf Business upgraden", "Upgrade to Business"],
  ["Checkout konnte nicht gestartet werden", "Checkout could not be started"],
  ["Pro-Abonnement kündigen? Der Zugriff bleibt bis zum Ende des Abrechnungszeitraums bestehen.", "Cancel Pro subscription? Access remains until the end of the billing period."],
  ["Pro-Abonnement dieses Arbeitsbereichs kündigen? Der Zugriff bleibt bis zum Ende des Abrechnungszeitraums bestehen.", "Cancel this workspace's Pro subscription? Access remains until the end of the billing period."],
  ["Abonnement gekündigt. Der Zugriff bleibt bis zum Ende des Abrechnungszeitraums bestehen.", "Subscription canceled. Access remains until the end of the billing period."],
  ["Kündigung fehlgeschlagen", "Cancellation failed"], ["Wird gekündigt...", "Canceling..."],
  ["Upgrade für unbegrenzte Videos und Aufnahmedauer.", "Upgrade for unlimited videos and recording duration."],
  ["Unbegrenzte Videos und Videolänge", "Unlimited videos and video length"],
  ["Alles aus Pro plus SSO und Zugriffskontrollen für Arbeitsbereiche", "Everything in Pro plus SSO and workspace access controls"],
  ["Dein Abonnement wurde gekündigt. Die Pro-Funktionen bleiben bis zum Ende des Abrechnungszeitraums verfügbar.", "Your subscription has been canceled. Pro features remain available until the end of the billing period."],
  ["Abonnement verwalten", "Manage subscription"], ["Abonnement kündigen", "Cancel subscription"],

  // Security remaining
  ["Die Passwörter stimmen nicht überein", "Passwords do not match"], ["Passwort konnte nicht aktualisiert werden", "Password could not be updated"],
  ["Die einzige Anmeldemethode kann nicht getrennt werden", "The only sign-in method cannot be disconnected"],
  ["API-Schlüssel konnte nicht erstellt werden", "API key could not be created"], ["API-Schlüssel konnte nicht gelöscht werden", "API key could not be deleted"],
  ["Konto wirklich löschen? Dies kann nicht rückgängig gemacht werden. Alle Videos und Daten werden dauerhaft gelöscht.", "Really delete the account? This cannot be undone. All videos and data will be permanently deleted."],
  ["Konto konnte nicht gelöscht werden", "Account could not be deleted"],
  ["Externe Konten, die mit deinem 99tools-Record-Konto verknüpft sind.", "External accounts linked to your 99tools Record account."],
  ["Erstelle API-Schlüssel für Integrationen wie Nextcloud. Schlüssel werden nur einmal direkt nach der Erstellung angezeigt.", "Create API keys for integrations such as Nextcloud. Keys are shown only once immediately after creation."],
  ["z. B. Mein Nextcloud", "e.g. My Nextcloud"],

  // Webhook event descriptions
  ["Neuerzeugung fehlgeschlagen", "Regeneration failed"],
  ["HTTP-POST-Benachrichtigungen für Video-Ereignisse empfangen (n8n, Zapier, eigene Systeme).", "Receive HTTP POST notifications for video events (n8n, Zapier, custom systems)."],
  ["— Ein Zuschauer hat ein Video angesehen", "— A viewer watched a video"], ["— Ein neuer Kommentar wurde veröffentlicht", "— A new comment was posted"],
  ["— Eine Emoji-Reaktion wurde hinzugefügt", "— An emoji reaction was added"], ["— Transkription abgeschlossen", "— Transcription completed"],
  ["— KI-Zusammenfassung abgeschlossen", "— AI summary completed"], ["— Ein CTA-Button wurde angeklickt", "— A CTA button was clicked"],
  ["— Testereignis aus den Einstellungen", "— Test event from settings"],

  // Organization settings
  ["Arbeitsbereich-Einstellungen", "Workspace settings"], ["Arbeitsbereich konnte nicht geladen werden", "Workspace could not be loaded"],
  ["Arbeitsbereich nicht gefunden", "Workspace not found"], ["Der Name des Arbeitsbereichs ist erforderlich", "Workspace name is required"],
  ["Arbeitsbereich aktualisiert", "Workspace updated"], ["Arbeitsbereich konnte nicht aktualisiert werden", "Workspace could not be updated"],
  ["Diesen Arbeitsbereich wirklich löschen? Dies kann nicht rückgängig gemacht werden. Alle Daten des Arbeitsbereichs werden dauerhaft gelöscht.", "Really delete this workspace? This cannot be undone. All workspace data will be permanently deleted."],
  ["Arbeitsbereich löschen", "Delete workspace"], ["Arbeitsbereich konnte nicht gelöscht werden", "Workspace could not be deleted"],
  ["Name des Arbeitsbereichs", "Workspace name"],
  ["Videos des Arbeitsbereichs nach einer festgelegten Anzahl von Tagen automatisch löschen. Angeheftete Videos sind ausgenommen.", "Automatically delete workspace videos after a set number of days. Pinned videos are excluded."],
  ["Diesen Arbeitsbereich und alle zugehörigen Daten dauerhaft löschen.", "Permanently delete this workspace and all associated data."],
  ["Mitglied konnte nicht entfernt werden", "Member could not be removed"], ["Rolle konnte nicht aktualisiert werden", "Role could not be updated"],
  ["E-Mail-Adresse ist erforderlich", "Email address is required"], ["Einladung konnte nicht gesendet werden", "Invitation could not be sent"],
  ["Einladung konnte nicht widerrufen werden", "Invitation could not be revoked"],
  ["Lade neue Mitglieder per E-Mail in diesen Arbeitsbereich ein.", "Invite new members to this workspace by email."],
  ["SSO-Einstellungen gespeichert", "SSO settings saved"], ["SSO-Einstellungen konnten nicht gespeichert werden", "SSO settings could not be saved"],
  ["SSO-Konfiguration entfernen? Mitglieder müssen sich anschließend mit Passwort anmelden.", "Remove SSO configuration? Members will then need to sign in with a password."],
  ["SSO entfernen", "Remove SSO"], ["SSO konnte nicht entfernt werden", "SSO could not be removed"],
  ["Token erstellt. Kopiere ihn jetzt – er wird später nicht erneut angezeigt.", "Token created. Copy it now — it will not be shown again."],
  ["Token konnte nicht erstellt werden", "Token could not be created"], ["Token konnte nicht widerrufen werden", "Token could not be revoked"],
  ["Token neu erzeugen", "Regenerate token"], ["Token widerrufen", "Revoke token"],
  ["SCIM-Token neu erzeugen? Der aktuelle Token funktioniert danach sofort nicht mehr.", "Regenerate SCIM token? The current token will immediately stop working."],
  ["SCIM-Token widerrufen? Die automatische Bereitstellung wird dadurch beendet.", "Revoke SCIM token? This will stop automatic provisioning."],
  ["Richte Single Sign-On für deinen Arbeitsbereich ein. Mitglieder können sich über deinen Identity Provider anmelden.", "Set up Single Sign-On for your workspace. Members can sign in through your identity provider."],
  ["Oder Metadaten-XML einfügen", "Or paste metadata XML"], ["Diese URL deinem IdP-Administrator bereitstellen", "Provide this URL to your IdP administrator"],
  ["Wenn dies erzwungen wird, müssen sich Mitglieder über deinen Identity Provider anmelden. Die Passwort-Anmeldung ist für Mitglieder des Arbeitsbereichs dann deaktiviert.", "When enforced, members must sign in through your identity provider. Password sign-in is disabled for workspace members."],
  ["Mitglieder des Arbeitsbereichs automatisch über deinen Identity Provider bereitstellen und entfernen.", "Automatically provision and remove workspace members through your identity provider."],
  ["Bearer-Token", "Bearer token"], ["Kopiere diesen Token jetzt. Er wird später nicht erneut angezeigt.", "Copy this token now. It will not be shown again."],
  ["SCIM-Status konnte nicht geladen werden", "SCIM status could not be loaded"],

  // Other exact legacy texts
  ["← Zurück", "← Back"], ["← Bibliothek", "← Library"], ["← Playlists", "← Playlists"],
  ["Keine Daten verfügbar.", "No data available."], ["Noch keine Kommentare.", "No comments yet."],

  // Final audit additions
  ["Einladung konnte nicht angenommen werden", "Invitation could not be accepted"], ["Anmeldung wird geprüft...", "Checking sign-in..."],
  ["Du wurdest zu einem Arbeitsbereich eingeladen. Melde dich an oder erstelle ein Konto, um die Einladung anzunehmen.", "You've been invited to a workspace. Sign in or create an account to accept the invitation."],
  ["Konto erstellen", "Create account"], ["Einladung wird angenommen...", "Accepting invitation..."],
  ["Du bist dem Arbeitsbereich beigetreten. Weiterleitung...", "You've joined the workspace. Redirecting..."], ["Einladung fehlgeschlagen", "Invitation failed"],
  ["Wir haben einen Bestätigungslink an", "We sent a confirmation link to"],
  ["gesendet. Klicke auf den Link, um dein Konto zu aktivieren. Der Link ist 24 Stunden gültig.", "Click the link to activate your account. The link is valid for 24 hours."],
  ["Wenn ein Konto mit dieser E-Mail-Adresse existiert, haben wir einen Link zum Zurücksetzen des Passworts gesendet. Der Link ist 1 Stunde gültig.", "If an account exists for this email address, we sent a password reset link. The link is valid for 1 hour."],
  ["Bestätigung fehlgeschlagen", "Confirmation failed"], ["E-Mail-Adresse wird bestätigt...", "Confirming email address..."],
  ["Dein Konto ist jetzt aktiv. Du kannst dich anmelden.", "Your account is now active. You can sign in."],
  ["Das Passwort muss mindestens 8 Zeichen lang sein", "Password must be at least 8 characters long"],
  ["Dieser Link zum Zurücksetzen des Passworts ist ungültig. Bitte fordere einen neuen an.", "This password reset link is invalid. Please request a new one."],
  ["Dein Passwort wurde erfolgreich zurückgesetzt.", "Your password has been reset successfully."], ["Passwort bestätigen", "Confirm password"],
  ["Nimm deinen Bildschirm auf oder lade ein Video hoch", "Record your screen or upload a video"],
  ["Teile den Link mit anderen", "Share the link with others"], ["Sieh Aufrufe und Feedback", "See views and feedback"],
  ["Passwort für dieses Video eingeben:", "Enter a password for this video:"], ["Zurück zur Bibliothek", "Back to library"],
  ["Video wird verarbeitet...", "Video is processing..."], ["Das dauert normalerweise ein bis zwei Minuten", "This usually takes one or two minutes"],
  ["Noch nicht gestartet", "Not started yet"], ["Wird transkribiert...", "Transcribing..."], ["Fehlgeschlagen", "Failed"],
  ["Noch nicht erstellt", "Not created yet"], ["Kommentar löschen", "Delete comment"],
  ["Webhook-URL konnte nicht gespeichert werden", "Webhook URL could not be saved"], ["Abonnement erfolgreich aktiviert!", "Subscription activated successfully!"],
  ["Erfolgreich verbunden", "Connected successfully"], ["API-Token", "API token"],

  ["Playlist-Optionen", "Playlist options"], ["Upload abgeschlossen", "Upload complete"], ["Kein Audio", "No audio"],
  ["Webhook-URL gespeichert", "Webhook URL saved"], ["Secret neu erzeugt", "Secret regenerated"],

];

const aliasToIndex = new Map<string, number>();
for (let i = 0; i < PAIRS.length; i++) {
  const [de, en] = PAIRS[i];
  aliasToIndex.set(de, i);
  aliasToIndex.set(en, i);
}

export function translateLegacyLiteral(value: string, language: UiLanguage): string {
  const idx = aliasToIndex.get(value);
  if (idx === undefined) return value;
  return PAIRS[idx][language === "de" ? 0 : 1];
}
