# SEO & GEO Report

🇬🇧 [English version](README.md)

Des rapports de visibilité **SEO** (Google Search Console), **trafic** (Google Analytics 4) et **GEO** (visibilité dans les assistants IA : ChatGPT, Perplexity, Gemini, AI Overviews…), bilingues FR/EN et très visuels, générés **avec Claude**.

- Vous exportez vos données (ou vous les collez / faites une capture), Claude les comprend, remplit le rapport et rédige l'analyse.
- Le serveur produit un **rapport HTML hébergé** (lien partageable), un **fichier HTML autonome** à télécharger et un **PDF** A4.
- **L'outil ne se connecte jamais à Google** : il ne fait que mettre en forme les données fournies.
- Fonctionne comme **serveur MCP** : connecteur personnalisé claude.ai (connexion OAuth automatique), Claude Desktop, Claude Code, ou en ligne de commande.
- Le favicon du site est récupéré automatiquement et intégré au rapport ; l'image Docker se met à jour toute seule (Watchtower).

![Aperçu du rapport](docs/screenshot-overview.png)

---

## Démarrage rapide (Docker)

```bash
mkdir seogeo && cd seogeo
curl -fsSLO https://raw.githubusercontent.com/flocom/SEO-GEO-Report/main/docker-compose.yml
docker compose up -d
docker compose logs seogeo      # affiche le mot de passe d'accès généré
```

(ou `git clone https://github.com/flocom/SEO-GEO-Report.git && cd SEO-GEO-Report && docker compose up -d`). L'image est `ghcr.io/flocom/seo-geo-report` (amd64 et arm64). Pour compiler depuis les sources : `docker compose -f docker-compose.yml -f docker-compose.build.yml up -d --build`.

C'est tout : **aucune variable n'est obligatoire**. Au premier démarrage, un mot de passe fort est généré, enregistré dans le volume de données (`/data/access-password.txt`) et affiché dans les logs à chaque démarrage :

```
  ACCESS PASSWORD: K79fGXWc1r-KrbXgzbFddyZN
```

Ouvrez <http://localhost:8080> : la page d'accueil affiche l'URL MCP détectée et les instructions de connexion.

Les variables optionnelles sont décrites dans [`.env.example`](.env.example) (copiez-le en `.env` si besoin) : `ACCESS_PASSWORD`, `PUBLIC_URL`, `PORT`, `AUTH_DISABLED`, `MCP_APPS`, `CHROME_PATH`, `UPDATE_CHECK`, `FAVICON_ALLOW_PRIVATE`, `WATCHTOWER_POLL_INTERVAL`, `OAUTH_REDIRECT_ALLOWLIST`.

### Mises à jour automatiques

Le `docker-compose.yml` inclut [Watchtower](https://github.com/nicholas-fedor/watchtower) (fork maintenu) : toutes les heures (`WATCHTOWER_POLL_INTERVAL`, en secondes), il vérifie si une nouvelle image `ghcr.io/flocom/seo-geo-report:latest` a été publiée et redémarre **uniquement** le conteneur seogeo (label `com.centurylinklabs.watchtower.enable=true`). Les données du volume sont conservées. Le serveur vérifie aussi les nouvelles versions sur GitHub toutes les 6 h et l'indique sur la page d'accueil et dans les logs (`UPDATE_CHECK=false` pour désactiver). Mise à jour manuelle : `docker compose pull && docker compose up -d`.

### Détection automatique de l'URL

Pas de `BASE_URL` à configurer : pour chaque requête, le serveur reconstruit son URL publique à partir des en-têtes `Forwarded` (RFC 7239), `X-Forwarded-Proto` / `X-Forwarded-Host` / `X-Forwarded-Port`, `CF-Visitor` (Cloudflare) puis `Host`. Les métadonnées OAuth, l'en-tête `WWW-Authenticate` et les liens des rapports utilisent donc automatiquement la bonne adresse, derrière n'importe quel reverse proxy. `PUBLIC_URL` permet de forcer une valeur si votre proxy ne transmet pas ces en-têtes.

## Exposer le serveur en HTTPS (requis pour claude.ai)

claude.ai doit pouvoir joindre le serveur via une URL publique en **HTTPS**.

**Cloudflare Tunnel (le plus simple, sans ouvrir de port)** — décommentez le service `cloudflared` dans `docker-compose.yml` :

- *Tunnel nommé* (URL stable, compte Cloudflare gratuit) : Zero Trust → Networks → Tunnels → créer un tunnel, mettez le jeton dans `.env` (`TUNNEL_TOKEN=...`) et ajoutez un « Public Hostname » (ex. `seo.mondomaine.fr`) vers `http://seogeo:8080`.
- *Tunnel rapide* (sans compte, pour tester) : URL `https://xxx.trycloudflare.com` visible dans `docker compose logs cloudflared` (elle change à chaque redémarrage).

**Caddy** (certificat automatique) :

```
seo.mondomaine.fr {
    reverse_proxy localhost:8080
}
```

**Traefik** (labels sur le service `seogeo`) :

```yaml
labels:
  - traefik.enable=true
  - traefik.http.routers.seogeo.rule=Host(`seo.mondomaine.fr`)
  - traefik.http.routers.seogeo.entrypoints=websecure
  - traefik.http.routers.seogeo.tls.certresolver=letsencrypt
  - traefik.http.services.seogeo.loadbalancer.server.port=8080
```

**nginx** : `proxy_pass http://127.0.0.1:8080;` avec `proxy_set_header Host $host;` et `proxy_set_header X-Forwarded-Proto $scheme;`.

## Connecter claude.ai

1. claude.ai → **Paramètres → Connecteurs → Ajouter un connecteur personnalisé** (Settings → Connectors → Add custom connector).
2. Nom : `SEO GEO Report`, URL : `https://seo.mondomaine.fr/mcp`. Laissez vides les champs OAuth avancés (enregistrement dynamique automatique).
3. Cliquez sur **Connecter** : une page de connexion du serveur s'ouvre, saisissez le mot de passe d'accès → **Autoriser**.
4. Dans une conversation, activez le connecteur (menu « Recherche et outils ») et demandez par exemple : *« Fais-moi le rapport SEO & GEO de monsite.fr pour le dernier trimestre »*. Le prompt **create_seo_geo_report** est aussi disponible.

La connexion survit aux redémarrages du conteneur (clients et jetons sont conservés, hachés, dans `/data/oauth.json`) ; les jetons d'accès durent 1 h et sont renouvelés automatiquement (rotation des refresh tokens, 90 jours).

Le connecteur est ensuite aussi disponible dans **Claude Desktop** et l'application mobile (même compte).

## Claude Code

```bash
claude mcp add --transport http seogeo https://seo.mondomaine.fr/mcp \
  --header "Authorization: Bearer VOTRE_MOT_DE_PASSE"
```

Le mot de passe d'accès est accepté directement comme jeton Bearer. Sans `--header`, lancez `/mcp` dans Claude Code pour vous connecter via OAuth. En local : `http://localhost:8080/mcp`.

## Claude Desktop (configuration manuelle)

Recommandé : ajoutez le connecteur comme sur claude.ai. Alternatives dans `claude_desktop_config.json` :

```json
{
  "mcpServers": {
    "seogeo": {
      "command": "npx",
      "args": ["-y", "mcp-remote", "https://seo.mondomaine.fr/mcp",
               "--header", "Authorization: Bearer VOTRE_MOT_DE_PASSE"]
    }
  }
}
```

ou entièrement en local (stdio, via Docker ou le binaire) :

```json
{
  "mcpServers": {
    "seogeo": {
      "command": "docker",
      "args": ["run", "-i", "--rm", "-v", "seogeo_seogeo-data:/data", "ghcr.io/flocom/seo-geo-report:latest", "mcp"]
    }
  }
}
```

(`seogeo_seogeo-data` = nom du volume créé par Compose, vérifiable avec `docker volume ls`.) En mode stdio, les rapports sont écrits dans le dossier de données et l'outil renvoie le chemin des fichiers (ou les URLs si `PUBLIC_URL` est défini).

## Déroulé typique avec Claude

> **Vous** : Fais-moi le rapport SEO & GEO de maison-lumen.fr pour juillet–septembre, comparé au trimestre précédent, en français.
>
> **Claude** : *(lit le guide)* Pour cela j'ai besoin de : 1) l'export Search Console (Performances → Résultats de recherche → dates « Comparer » → Exporter) ; 2) l'export GA4 (Acquisition de trafic par groupe de canaux et par source/support, pages de destination) ; 3) si vous en avez, quelques tests dans ChatGPT / Perplexity / Gemini : votre site est-il cité ?
>
> **Vous** : *(joint le zip Search Console, deux CSV GA4 et colle un tableau de tests)*
>
> **Claude** : *(create_report → import_search_console_csv × 6 → import_analytics_csv × 2 → set_geo → set_narrative → validate_report → render_report)* Voici votre rapport : lien en ligne, HTML et PDF. Points clés : clics Google +36 %, trafic depuis les IA ×2,6 (ChatGPT en tête), CTR en baisse sur les pages les plus vues…

Outils MCP exposés : `get_report_guide`, `create_report`, `update_meta`, `set_search_console`, `set_analytics`, `set_geo`, `set_narrative`, `add_text_section`, `update_text_section`, `remove_text_section`, `set_options`, `import_search_console_csv`, `import_analytics_csv`, `validate_report`, `render_report`, `generate_report`, `get_report`, `list_reports`, `duplicate_report`, `delete_report` ; prompt `create_seo_geo_report` ; ressources `seogeo://guide`, `seogeo://schema`, `seogeo://example/fr`, `seogeo://example/en`.

**MCP Apps** : sur les hôtes compatibles, `render_report` / `generate_report` affichent le rapport directement dans la conversation (ressource `ui://seogeo/report-viewer.html`). Désactivable avec `MCP_APPS=false` ; les autres hôtes reçoivent simplement les liens.

## Exports : HTML et PDF

| Lien | Contenu |
|---|---|
| `/r/<jeton>` | rapport en ligne (`?lang=en` pour l'autre langue) |
| `/r/<jeton>/download` | fichier HTML autonome (CSS et graphiques SVG intégrés, aucune dépendance) |
| `/r/<jeton>.pdf` (ou `/r/<jeton>/pdf`) | PDF A4 (`?download=1` pour forcer le téléchargement) |

Le PDF est produit par Chromium sans interface (inclus dans l'image Docker avec les polices Noto / DejaVu / emoji), mis en cache dans le dossier de données et régénéré quand le rapport change. Hors Docker, Chromium ou Google Chrome est détecté automatiquement (`CHROME_PATH` pour forcer) ; s'il est absent, l'export HTML fonctionne toujours et l'erreur PDF est expliquée. `render_report` accepte `format` = `html` (défaut), `pdf` ou `both`, et `include_html` / `include_pdf` pour joindre le document à la réponse.

## Ligne de commande

```bash
seogeo example --lang fr > data.json        # rapport de démonstration complet
seogeo schema > schema.json                 # schéma JSON du format
seogeo validate -i data.json                # erreurs, avertissements, complétude
seogeo render -i data.json -o rapport.html  # HTML autonome
seogeo render -i data.json -o rapport.pdf   # PDF (Chromium requis)
seogeo render -i - --lang en < data.json > report.html
seogeo import-gsc export-gsc.zip -o gsc.json            # zip ou dossier de CSV Search Console
seogeo import-gsc export-gsc.zip --into data.json -o data.json
seogeo import-ga4 canaux.csv sources.csv --into data.json -o data.json
seogeo serve            # serveur HTTP (MCP /mcp, OAuth, hébergement des rapports)
seogeo mcp              # serveur MCP stdio
seogeo version
```

Des binaires (Linux, macOS, Windows ; amd64/arm64) sont joints à chaque [release](https://github.com/flocom/SEO-GEO-Report/releases). Compilation : `go build -o seogeo ./cmd/seogeo` (Go 1.26). Avec Docker : `docker compose run --rm seogeo render -i /data/data.json -o /data/rapport.pdf`. `render --no-favicon` désactive la récupération du favicon.

## Format des données

Un rapport est un document JSON (`seogeo schema` pour le schéma complet, `examples/demo-fr.json` et `examples/demo-en.json` pour des exemples) :

- `meta` : site, période, période de comparaison, langue (`fr`/`en`), auteur, client, logo, couleur, favicon (`favicon_url` : vide = récupéré automatiquement depuis `site_url`, `none` = aucun) ;
- `search_console` : totaux (clics, impressions, CTR, position), séries quotidiennes, requêtes, pages, pays, appareils, apparence, marque/hors marque, indexation, Core Web Vitals ;
- `analytics` : totaux (tous canaux et recherche organique), séries quotidiennes, canaux, sources/supports (les assistants IA y sont détectés automatiquement), pages de destination ;
- `geo` : trafic venant des IA, tests de citations, part de voix, AI Overviews, robots IA ;
- `narrative` : synthèse, points forts, points d'attention, actions, recommandations, commentaires par section ;
- `sections` : sections Markdown libres ; `options` : affichage.

Conventions : **pourcentages en pourcent** (3,2 % → `3.2`), dates `AAAA-MM-JJ`, durées en secondes, chaque indicateur sous la forme `{"current": …, "previous": …}` (la valeur précédente alimente les visuels de progression).

## Sécurité

- `/mcp` exige un jeton Bearer : jeton OAuth émis par le serveur (OAuth 2.1, PKCE S256 obligatoire, enregistrement dynamique limité aux callbacks claude.ai / claude.com / localhost, codes à usage unique valables 5 min, jetons opaques stockés hachés en SHA-256) **ou** le mot de passe d'accès. Les mauvais mots de passe sont limités (8 essais / 15 min par IP).
- Utilisez un mot de passe long si vous définissez `ACCESS_PASSWORD` (le mot de passe généré fait 24 caractères aléatoires). Pour le changer, modifiez/supprimez `access-password.txt` dans le volume ou définissez `ACCESS_PASSWORD`, puis redémarrez. Supprimer `/data/oauth.json` révoque tous les clients.
- **Les rapports publiés (`/r/<jeton>`) ne demandent pas d'authentification** : l'URL contient un jeton aléatoire de 192 bits impossible à deviner. Toute personne disposant du lien peut lire le rapport : partagez-le comme un lien « accessible à toute personne disposant du lien ». Supprimer le rapport (`delete_report`) désactive le lien. Les pages sont marquées `noindex`.
- `AUTH_DISABLED=true` ouvre le serveur à quiconque peut le joindre : à réserver à un usage local.
- Les en-têtes `X-Forwarded-*` sont pris en compte sans liste de proxys de confiance (pour la détection automatique) ; ils n'influencent que les URLs renvoyées à celui qui les envoie.
- Le conteneur tourne en utilisateur non-root ; les données sont dans le volume `seogeo-data` (`/data`).
- La récupération du favicon refuse les adresses privées, locales et de métadonnées cloud (protection SSRF, `FAVICON_ALLOW_PRIVATE=true` pour un intranet), limite la taille des fichiers et rejette les SVG contenant du script ou des références externes.
- Watchtower a accès au socket Docker : retirez le service `watchtower` si vous préférez mettre à jour manuellement.
