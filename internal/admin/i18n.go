package admin

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

// messages holds the French and English texts of the admin pages.
var messages = map[string][2]string{
	"title":          {"Mises à jour", "Updates"},
	"subtitle":       {"Administration du serveur SEO & GEO Report", "SEO & GEO Report server administration"},
	"home":           {"Accueil", "Home"},
	"lang_switch":    {"English", "Français"},
	"logout":         {"Se déconnecter", "Sign out"},
	"login_title":    {"Connexion à l'administration", "Admin sign in"},
	"password_label": {"Mot de passe d'accès", "Access password"},
	"login_btn":      {"Se connecter", "Sign in"},
	"login_help": {"Le même mot de passe que pour connecter Claude : il est affiché dans les logs au démarrage (docker compose logs seogeo), enregistré dans access-password.txt du volume de données, ou défini par ACCESS_PASSWORD.",
		"The same password as for connecting Claude: printed in the logs at startup (docker compose logs seogeo), stored in access-password.txt in the data volume, or set with ACCESS_PASSWORD."},
	"wrong_password":     {"Mot de passe incorrect.", "Wrong password."},
	"too_many":           {"Trop de tentatives. Réessayez dans %d min.", "Too many attempts. Try again in %d min."},
	"auth_disabled_warn": {"Authentification désactivée (AUTH_DISABLED=true) : toute personne qui peut joindre ce serveur peut utiliser cette page.", "Authentication is disabled (AUTH_DISABLED=true): anyone who can reach this server can use this page."},

	"sec_version":     {"Version installée", "Installed version"},
	"current_version": {"Version", "Version"},
	"build":           {"Build", "Build"},
	"uptime":          {"En service depuis", "Uptime"},
	"started":         {"démarré le %s", "started on %s"},

	"sec_latest":       {"Dernière version publiée", "Latest release"},
	"published":        {"Publiée le", "Published on"},
	"notes":            {"Notes de version", "Release notes"},
	"notes_truncated":  {"(extrait — voir la suite sur GitHub)", "(excerpt — read more on GitHub)"},
	"view_release":     {"Voir la version sur GitHub", "View the release on GitHub"},
	"status":           {"État", "Status"},
	"status_uptodate":  {"À jour", "Up to date"},
	"status_available": {"Mise à jour disponible", "Update available"},
	"status_unknown":   {"Inconnu (pas encore vérifié)", "Unknown (not checked yet)"},
	"status_dev":       {"Version de développement (%s) : comparaison impossible", "Development build (%s): cannot compare"},
	"last_check":       {"Dernière vérification", "Last check"},
	"never":            {"jamais", "never"},
	"check_error":      {"échec : %s", "failed: %s"},
	"auto_check_on":    {"Vérification automatique sur GitHub toutes les 6 h.", "Automatic GitHub check every 6 hours."},
	"auto_check_off":   {"Vérification automatique désactivée (UPDATE_CHECK=false) : utilisez le bouton.", "Automatic check disabled (UPDATE_CHECK=false): use the button."},
	"btn_check":        {"Rechercher les mises à jour", "Check for updates"},

	"sec_update":    {"Mise à jour", "Update"},
	"btn_update":    {"Mettre à jour maintenant", "Update now"},
	"btn_force":     {"Forcer la mise à jour", "Force update"},
	"update_help":   {"Watchtower télécharge la nouvelle image puis redémarre le service (1 à 3 minutes, les rapports et le mot de passe sont conservés).", "Watchtower pulls the new image and restarts the service (1 to 3 minutes; reports and password are kept)."},
	"force_warning": {"Aucune nouvelle version n'est annoncée : Watchtower va seulement vérifier l'image sur le registre et redémarrer le service si elle a changé.", "No new version is announced: Watchtower will only check the image on the registry and restart the service if it changed."},

	"auto_updates":      {"Mises à jour automatiques (Watchtower)", "Automatic updates (Watchtower)"},
	"wt_ok":             {"Configurées et joignables", "Configured and reachable"},
	"wt_not_configured": {"Non configurées pour la mise à jour en un clic (aucun jeton d'API Watchtower)", "Not configured for one-click updates (no Watchtower API token)"},
	"wt_unreachable":    {"Watchtower injoignable", "Watchtower unreachable"},
	"wt_unauthorized":   {"Jeton refusé par Watchtower", "Token rejected by Watchtower"},
	"wt_error":          {"Réponse inattendue de Watchtower", "Unexpected answer from Watchtower"},
	"wt_last_job":       {"Dernière demande depuis cette page : %s — %d conteneur(s) analysé(s), %d mis à jour, %d en échec.", "Last request from this page: %s — %d container(s) scanned, %d updated, %d failed."},
	"wt_hint_unauthorized": {"Le jeton lu par seogeo diffère de celui de Watchtower : redémarrez les deux services (docker compose up -d --force-recreate).",
		"The token read by seogeo differs from Watchtower's: restart both services (docker compose up -d --force-recreate)."},

	"manual_title":  {"Mettre à jour manuellement", "Update manually"},
	"manual_docker": {"Sur le serveur, dans le dossier du docker-compose.yml :", "On the server, in the folder of docker-compose.yml:"},
	"enable_title":  {"Activer la mise à jour en un clic", "Enable one-click updates"},
	"enable_help": {"Reprenez le docker-compose.yml fourni avec le projet (v0.3.0 ou plus), ou ajoutez ceci à votre fichier puis lancez docker compose up -d :",
		"Use the docker-compose.yml shipped with the project (v0.3.0 or later), or add this to your file, then run docker compose up -d:"},
	"source_title": {"Version compilée depuis les sources", "Built from source"},
	"source_help": {"Cette instance a été construite localement (docker-compose.build.yml ou go build) : Watchtower ne la met pas à jour. Pour mettre à jour, dans le dossier du projet :",
		"This instance was built locally (docker-compose.build.yml or go build): Watchtower does not update it. To update, in the project folder:"},
	"binary_title": {"Exécutable lancé hors Docker", "Binary running outside Docker"},
	"binary_help": {"Téléchargez la dernière version depuis GitHub Releases, remplacez l'exécutable puis redémarrez le service.",
		"Download the latest version from GitHub Releases, replace the binary and restart the service."},

	"flash_checked":             {"Vérification effectuée.", "Check done."},
	"flash_check_wait":          {"Vérification déjà faite il y a moins d'une minute : réessayez dans %d s.", "Already checked less than a minute ago: try again in %d s."},
	"flash_check_failed":        {"Impossible d'interroger GitHub (détail ci-dessous).", "Could not query GitHub (details below)."},
	"flash_update_busy":         {"Une mise à jour est déjà en cours.", "An update is already running."},
	"flash_update_wait":         {"Mise à jour déjà demandée il y a moins de 30 s.", "An update was already requested less than 30 s ago."},
	"flash_update_failed":       {"Watchtower a refusé la demande : consultez docker compose logs watchtower.", "Watchtower rejected the request: see docker compose logs watchtower."},
	"flash_update_unreachable":  {"Watchtower est injoignable.", "Watchtower is unreachable."},
	"flash_update_unauthorized": {"Watchtower a refusé le jeton d'API.", "Watchtower rejected the API token."},
	"flash_update_unavailable":  {"La mise à jour en un clic n'est pas disponible sur cette installation.", "One-click update is not available on this installation."},
	"flash_logged_out":          {"Vous êtes déconnecté.", "You are signed out."},
	"flash_csrf":                {"Formulaire expiré ou invalide : veuillez réessayer.", "Expired or invalid form: please try again."},

	"progress_started":    {"Mise à jour lancée… Watchtower télécharge l'image puis redémarre le service (1 à 3 minutes). Cette page se met à jour toute seule.", "Update started… Watchtower pulls the image then restarts the service (1 to 3 minutes). This page updates by itself."},
	"progress_restarting": {"Redémarrage du service…", "Service restarting…"},
	"progress_done":       {"Mis à jour vers v%s.", "Updated to v%s."},
	"progress_restarted":  {"Le service a redémarré, version inchangée (v%s).", "The service restarted, same version (v%s)."},
	"progress_nothing":    {"Watchtower n'a trouvé aucune image plus récente : rien à mettre à jour.", "Watchtower found no newer image: nothing to update."},
	"progress_failed":     {"Watchtower signale un échec : consultez docker compose logs watchtower.", "Watchtower reports a failure: see docker compose logs watchtower."},
	"progress_timeout": {"Aucun changement après 5 minutes. Vérifiez : 1) docker compose logs watchtower (la demande a-t-elle été reçue ?) ; 2) qu'une nouvelle image existe sur ghcr.io/flocom/seo-geo-report ; 3) que le conteneur seogeo porte le label com.centurylinklabs.watchtower.enable=true. Mise à jour manuelle : docker compose pull && docker compose up -d",
		"No change after 5 minutes. Check: 1) docker compose logs watchtower (was the request received?); 2) that a new image exists on ghcr.io/flocom/seo-geo-report; 3) that the seogeo container has the label com.centurylinklabs.watchtower.enable=true. Manual update: docker compose pull && docker compose up -d"},
	"progress_noscript": {"JavaScript est désactivé : rechargez cette page dans une ou deux minutes pour voir la nouvelle version.", "JavaScript is disabled: reload this page in a minute or two to see the new version."},
	"reload":            {"Recharger la page", "Reload the page"},

	"api_hint": {"Automatisation : GET /admin/api/status et POST /admin/api/update avec l'en-tête Authorization: Bearer <mot de passe>.", "Automation: GET /admin/api/status and POST /admin/api/update with the header Authorization: Bearer <password>."},
}

// pickLang chooses the language: ?lang=, then the lang cookie, then
// Accept-Language (French when preferred, English otherwise).
func pickLang(r *http.Request) string {
	if l := normLang(r.URL.Query().Get("lang")); l != "" {
		return l
	}
	if c, err := r.Cookie(langCookie); err == nil {
		if l := normLang(c.Value); l != "" {
			return l
		}
	}
	return acceptLang(r.Header.Get("Accept-Language"))
}

func normLang(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "fr":
		return "fr"
	case "en":
		return "en"
	}
	return ""
}

// acceptLang returns the preferred of fr/en in an Accept-Language header.
func acceptLang(h string) string {
	best, bestQ := "fr", -1.0
	for _, part := range strings.Split(h, ",") {
		tag, params, _ := strings.Cut(strings.TrimSpace(part), ";")
		tag = strings.ToLower(strings.TrimSpace(tag))
		q := 1.0
		if v, ok := strings.CutPrefix(strings.TrimSpace(params), "q="); ok {
			if _, err := fmt.Sscanf(v, "%g", &q); err != nil {
				q = 0
			}
		}
		lang := ""
		switch {
		case tag == "fr" || strings.HasPrefix(tag, "fr-"):
			lang = "fr"
		case tag == "en" || strings.HasPrefix(tag, "en-"):
			lang = "en"
		}
		if lang != "" && q > bestQ {
			best, bestQ = lang, q
		}
	}
	if bestQ < 0 && strings.TrimSpace(h) != "" {
		return "en" // neither French nor English requested: English
	}
	return best
}

// tr translates a message key.
func tr(lang, key string, args ...any) string {
	m, ok := messages[key]
	if !ok {
		return key
	}
	s := m[0]
	if lang == "en" {
		s = m[1]
	}
	if len(args) > 0 {
		s = fmt.Sprintf(s, args...)
	}
	return s
}

func formatTime(lang string, t time.Time) string {
	if t.IsZero() {
		return tr(lang, "never")
	}
	t = t.Local()
	if lang == "en" {
		return t.Format("Jan 2, 2006 15:04 MST")
	}
	return t.Format("02/01/2006 15:04 MST")
}

func formatDate(lang string, t time.Time) string {
	if t.IsZero() {
		return ""
	}
	if lang == "en" {
		return t.Format("January 2, 2006")
	}
	months := [...]string{"janvier", "février", "mars", "avril", "mai", "juin", "juillet", "août", "septembre", "octobre", "novembre", "décembre"}
	return fmt.Sprintf("%d %s %d", t.Day(), months[t.Month()-1], t.Year())
}

func formatDuration(lang string, d time.Duration) string {
	d = d.Truncate(time.Minute)
	days := int(d.Hours()) / 24
	h := int(d.Hours()) % 24
	m := int(d.Minutes()) % 60
	day := "j"
	if lang == "en" {
		day = "d"
	}
	switch {
	case days > 0:
		return fmt.Sprintf("%d %s %d h %d min", days, day, h, m)
	case h > 0:
		return fmt.Sprintf("%d h %d min", h, m)
	default:
		return fmt.Sprintf("%d min", m)
	}
}
