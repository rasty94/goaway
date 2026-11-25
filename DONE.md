# GoAway - Tareas Completadas

Ultima actualizacion: 2026-11-25

Las tareas pendientes viven en [TODO.md](TODO.md). Cuando una tarea se termina, se mueve aqui con sus criterios de aceptacion.

---

## DX, CI/CD y Operacion
- [x] Cobertura y linters en CI (incluyendo golangci-lint).
- [x] Targets de auditoría de seguridad en Makefile (gosec, govulncheck, trivy, gitleaks).
- [x] Integración de SonarQube (Unified Analysis + Coverage).
- [x] Firma/checksums en release.
- [x] Especificacion OpenAPI/Swagger.
- [x] Guia para ejecutar sin root en puerto 53.
- [x] Apagado graceful (SIGINT/SIGTERM) en ciclo de vida.
- [x] Smoke E2E en Docker para validacion de endpoints criticos.
- [x] Sincronización completa con Upstream (pommee/goaway v0.63+).
- [x] Migración automática de resoluciones YAML a SQLite.
- [x] Hardening de seguridad v0.92.0 (gosec, permissions, slowloris, cmd injection).
- [x] Auditoria v0.93.0: gosec 0 findings, govulncheck 0 vulnerabilidades, trivy limpio.
- [x] Escaneo de dependencias y vulnerabilidades en el workflow de PR.
- [x] Contenedor ejecutando como usuario no root con file capabilities.
- [x] settings.yaml fuera del repo (contenia un jwtSecret real).
- [x] `jwtSecret` filtrado dado por quemado: no se purga el historial (romperia clones y forks); las instancias que arrancan con el se rotan solas.
- [x] Release v1.0.0 (2026-11-25): tag `v1.0.0`, binarios GoReleaser en GitHub Releases e imagen multi-arch (amd64/arm64) en `ghcr.io/rasty94/goaway:1.0.0` y `:latest`. La imagen se publico a mano con `make publish` porque el job fallo (permisos del paquete en ghcr, ya corregidos).
- [x] Build de release mas rapido: imagen Docker con compilacion cruzada en vez de QEMU; GoReleaser sin ppc64le/s390x/windows-arm64 y con reruns idempotentes.
- [x] Referencias al upstream (`pommee/goaway`) pasadas al fork: installer/updater, comprobacion de version (ahora via `releases/latest`, ignora prereleases), changelog de la UI, docs y README. GoReleaser marca los tags `-rc` como prerelease.

## Frontend y UX
- [x] Localizacion completa (EN/ES).
- [x] Cola de consultas en vivo (WebSocket multi-cliente).
- [x] Visualizaciones avanzadas de actividad.
- [x] Grafo de topologia de clientes.
- [x] Endurecimiento mobile-first en rutas clave.
- [x] Dashboard HTMX alpha sin dependencia Node para ese modo.
- [x] Eliminacion de todos los `any` del cliente; tipos reales.

## DNS y Politicas Base
- [x] Capa de cache DNS con TTL y control desde UI.
- [x] DNS local y registros CNAME/A/AAAA.
- [x] Allowlist completa.
- [x] Wildcards en allow/block.
- [x] Bloqueo por regex.
- [x] Reenvio condicional por dominio.
- [x] Horarios de politica que cruzan medianoche.

## Arquitectura, Backup y Observabilidad
- [x] Export/Import tipo Teleporter (settings + base de datos).
- [x] Backups remotos (S3, WebDAV, local montado).
- [x] Metricas Prometheus en /metrics.

## Seguridad y Control de Acceso
- [x] Multiusuario para administracion.
- [x] Rate limiting por cliente con metrica dedicada.
- [x] Gestion de grupos por cliente (IP/MAC) con enforcement en resolucion.

## Red y Plataforma
- [x] Servidor DHCPv4 nativo.
- [x] Gestion web de DHCP.
- [x] Leases estaticos DHCP.
- [x] HA Fase 1: sincronizacion pasiva Primary -> Replica por backup + teleporter (manual y programada).
- [x] Soporte total Windows/macOS (de Beta a Full).

---

## EPIC-01 Secure Resolver (P0)

- [x] FEATURE: Soporte de upstream DoT saliente
  - Multiples upstream DoT con SNI y validacion de certificado.
  - Health checks periodicos y salida del pool en fallo.
  - Metricas por upstream: latencia, timeout, errores TLS.

- [x] FEATURE: Soporte de upstream DoH saliente
  - DoH sobre HTTP/2.
  - Bootstrap sin dependencia circular.
  - Fallback ordenado entre UDP/TCP/DoT/DoH.

- [x] FEATURE: DNSSEC completo
  - Validacion de cadena de confianza.
  - Estado secure/insecure/bogus en logs y API.
  - Diagnostico por dominio con endpoint dedicado.

- [x] FEATURE: Cache serve-stale + prefetch inteligente
  - Responder con cache expirada cuando upstreams fallen.
  - Prefetch de entradas calientes antes de expirar.
  - Metricas separadas para hit, stale-hit y prefetch-hit.

## EPIC-02 Policy Engine (P0)

- [x] FEATURE: Motor jerarquico de politicas
  - Precedencia explicita: global > grupo > subred > cliente.
  - Condiciones por IP, MAC, CIDR, hostname, dia y franja horaria.
  - API devuelve regla ganadora y origen.

- [x] FEATURE: Filtrado por categorias
  - Categorias iniciales: ads, trackers, malware, adult, gambling, social.
  - Activacion por grupo y por horario.
  - Conteos por categoria y fuente.

- [x] FEATURE: SafeSearch y modos restringidos
  - Forzado en motores principales por grupo.
  - Excepciones por cliente/subred.
  - Resultado visible en simulador.

## EPIC-03 Explainability (P0)

- [x] FEATURE: Endpoint de explicacion DNS
  - Endpoint de explicacion con fqdn, tipo, cliente y timestamp.
  - Devuelve matching (listas, wildcard, regex), upstream y TTL final.
  - Indica cache-hit o stale-hit.

- [x] FEATURE: Simulador de politicas en UI
  - Formulario dominio + cliente + fecha/hora.
  - Arbol de evaluacion y regla final.
  - Comparativa actual vs draft.

- [x] FEATURE: Dry-run para nuevas reglas
  - Dry-run no bloquea, solo registra impacto.
  - Panel con posibles falsos positivos.
  - Exportacion CSV para revision.

## EPIC-04 Privacy & Audit (P1)

- [x] FEATURE: Retencion por capas
  - Retencion separada para raw logs, agregados y metricas.
  - Purga automatica visible en UI.
  - Configuracion por tipo de dato.

- [x] FEATURE: Anonimizacion IP
  - Truncado IPv4/IPv6 en almacenamiento/export.
  - Modo reversible solo con clave activa.
  - Aviso de impacto en troubleshooting.

- [x] FEATURE: Auditoria administrativa
  - Registra login, cambios de config, import/restore y reglas.
  - Middleware automatico para toda la API.
  - Evento con actor, timestamp y accion.

## EPIC-05 High Availability (Active Clustering) (P1)

- [x] Cluster logic and Leader Election (Priority-based)
- [x] Global Replication Manager (Push-based)
- [x] Blacklist/Whitelist/Group Replicators
- [x] Replication API Endpoints
- [x] Cluster Management Dashboard in React (UI Unified)
- [x] Project Version Bump to v0.90.0
- [x] Phase 2: DHCP & Leases Replication
- [x] Phase 3: Virtual IP (VIP) Floating & DNS Proxy Balancing (Sticky Sessions)
- [x] FEATURE: Modo despliegue con VIP flotante (Linux / macOS)
  - Health checks expulsan nodo en fallo DNS/DB.
  - Runbook de failover y rollback.
- [x] Phase 4: Cluster Auth & Security Signatures (2026-09-07)
  - Firma HMAC-SHA256 (timestamp + body) con `highAvailability.secret` en heartbeat y replicate.
  - Ventana anti-replay de 30 s; rechazo 401 con entrada en el audit log.
  - Fail closed sin secreto configurado. Tests en `cluster/signing_test.go` y `api/api_ha_test.go`.
- [x] FEATURE: Replicacion en tiempo real (2026-09-07)
  - Outbox persistente en SQLite (`replication_outbox`), una fila por peer, entrega en orden y reintento cada 5 s hasta ack.
  - Replica listas, grupos (flags y dominios por grupo), asignaciones de clientes, DNS local, leases DHCPv4/v6 y estaticos.
  - Replicadores de grupo y resolucion cableados en el registry (antes no se llamaba a SetReplicator).
  - Test `cluster/replication_test.go`: cola mientras el peer esta caido y entrega ordenada al volver.

## Correcciones de la revision del 2026-09-07

- [x] Upstreams: la API de `/api/upstream` escribia en `dns.upstream.fallback`, campo legado que el resolver nunca lee, asi que anadir un upstream desde la interfaz no tenia efecto y la lista aparecia vacia. Ahora opera sobre `servers`; "preferido" es la primera posicion. Tests en `api/upstream_test.go`.
- [x] Import: lista negra en una sola transaccion (`AddCustomDomains`), dominios por grupo en lote con un solo `RefreshCache` por grupo, zip leido en streaming y gravity.db volcado a disco sin pasar por memoria. Evento `blacklist.add.batch` para replicar el lote.

- [x] HA: test de integracion con dos nodos reales (`api/cluster_integration_test.go`): replicacion extremo a extremo, rechazo de firmas invalidas, 409 en split-brain, eleccion por prioridad y recuperacion del outbox tras una caida. Descubrio una condicion de carrera real en el rol del nodo, ya corregida.
- [x] HA: backlog de replicacion visible por nodo (`pendingEvents`, `needsResync`) en la API y el dashboard, con log de error al descartar eventos.

- [x] HA: los rechazos en los endpoints de cluster ya no escriben una fila de audit por peticion; como maximo una por minuto.
- [x] Banner de arranque: muestra el primer servidor configurado en vez del campo legado `preferred`, siempre vacio.
- [x] HA: `selfRole` protegido por su propio mutex. La goroutine de eleccion lo escribia mientras las peticiones lo leian sin lock, asi que un nodo degradado podia seguir aceptando eventos replicados.

## EPIC-06 Ecosystem (P1)

- [x] FEATURE: Wizard de importacion Pi-hole (2026-09-07)
  - Solo Pi-hole v6 (zip Teleporter con gravity.db y pihole.toml).
  - Importa adlists, listas negra/blanca, grupos, dominios por grupo, clientes (IP/MAC), DNS local (hosts y CNAME), upstreams y leases DHCP estaticos.
  - Vista previa con conflictos marcados por seccion y reporte final con creadas, omitidas y errores.
  - Regex de Pi-hole en forma wildcard se convierten a `*.dominio`; el resto solo en listas de grupo. Clientes por subred, hostname o interfaz se reportan como no soportados.
  - Test `pihole/parse_test.go` con un archivo sintetico.

- [x] FEATURE: API keys con scopes
  - Scopes: read, write, backup, metrics, admin.
  - Expiracion, revocacion y nombre descriptivo.
  - Ultimo acceso y logs de uso.

## EPIC-08 DHCPv6 y Router Advertisements (P2)

- [x] FEATURE: DHCPv6 nativo + Router Advertisements
  - Asignacion dinamica de IPv6.
  - Soporte para RA (Router Advertisements) para SLAAC.
  - Gestion de leases IPv6 en UI.

## EPIC-10 Premium Native Dashboard

- [x] Modern layout with sidebar and glassmorphism design
- [x] Dual-stack DHCPv4/v6 monitoring view
- [x] Hierarchical DNS Policy management view
- [x] Interactive DNS Resolution & Wildcard console
- [x] Real-time activity stream and policy explainer

## Foundations Tecnicas Transversales

- [x] TECH: Framework de migraciones de esquema
  - Migraciones versionadas con up/down.
  - Runner idempotente en startup.
  - Estado y rollback por pasos implementados.

- [x] TECH: Esquema versionado de configuracion + upgrade hooks
  - Campo de version de configuracion.
  - Hooks de upgrade/validacion al cargar settings.
  - Persistencia automatica de upgrades compatibles.

## Correcciones de la revision del 2026-11-24

- [x] SECURITY: Resueltas las 7 vulnerabilidades de `govulncheck` actualizando `golang.org/x/net` a `v0.63.0` (junto con `golang.org/x/crypto` y demas `golang.org/x/*`) y `github.com/gorilla/websocket` a `v1.5.4-0.20240701034025-d67f41855da4`. Verificado con `govulncheck ./...` (0 vulnerabilidades).
- [x] SECURITY: Corregido hallazgo `gosec` G104 en `backend/pihole/parse.go:163` devolviendo `errors.Join(err, tmp.Close())`. Verificado con `gosec -quiet ./backend/...` (0 hallazgos).
- [x] PERF: Pool persistente de `http.Client`/`http.Transport` con HTTP/2 en `DNSServer` para consultas upstream DoH (`backend/dns/server/upstream_exchange.go`), con contexto de cancelacion, limite de lectura de 64 KiB, drenado del body en errores y `CloseIdleConnections()` en shutdown/restart. Test de reutilizacion HTTP/2 en `backend/dns/server/upstream_exchange_test.go`.
- [x] TECH: Limpiados los 8 warnings de `react-hooks/exhaustive-deps` en `client/src/` (`eslint` con 0 errores y 0 warnings).
- [x] CHORE: Alineados `PROXY_PORT=5354` en `Dockerfile`, imagen `v0.93.0` en `docker-compose.yml`, version `0.93.0` en `client/package.json` y badges en `README.md`.
- [x] FEATURE: Extended DNS Errors (RFC 8914 / EDE) configurable en `dns.edeEnabled` (`backend/dns/server/ede.go`): adjunta EDNS0 EDE `Filtered` en respuestas bloqueadas (con categoria si aplica), `DNSBogus` en fallos DNSSEC en modo `strict` y `StaleAnswer` en respuestas serve-stale. Verificado con tests de serializacion OPT en `backend/dns/server/ede_test.go`.
- [x] FEATURE: Estrategias de seleccion de upstreams (`strict`, `parallel_best`, `random`) en `dns.upstream.strategy` (`backend/dns/server/upstream_strategy.go`), con cancelacion de consultas perdedoras en `parallel_best`, salto de `SERVFAIL` si otro upstream responde OK, metricas `goaway_dns_upstream_wins_total` y `goaway_dns_upstream_latency_seconds`, y tests en `backend/dns/server/upstream_strategy_test.go`.
- [x] FEATURE: Soporte DoQ (DNS over QUIC, RFC 9250) en cliente upstream (con pool de sesiones QUIC, ALPN `doq`, framing 2 bytes y ID=0 en wire) y servidor opcional configurable en `dns.ports.doq` (`backend/dns/server/doq.go`, integrado en `ServiceRegistry`, `lifecycle.Manager` y `Application`). Test de integracion extremo a extremo en `backend/dns/server/doq_test.go`.
- [x] TECH: Anadidos tests unitarios en `backend/user/service_test.go`, `backend/api/key/service_test.go`, `backend/resolution/service_test.go` y `backend/dhcp/service_test.go` (DHCPv4/v6 + leases estaticos). Al probar DHCPv6 se detecto y corrigio un bug de nombres de columna GORM (`duid`/`iaid` vs `du_id`/`ia_id`) en `backend/dhcp/repository.go`.
- [x] TECH: Anadido Vitest como runner de tests en `client/` (`pnpm -C client test`), integrado en `.github/workflows/pull-request.yml`, con tests iniciales en `client/src/pages/validation.test.ts` y `client/src/lib/utils.test.ts`.
- [x] HA: Cache de nonces anti-replay en la ventana de 30 s (`cluster.NonceCache` + `HeaderNonce`) y firma HMAC de las respuestas de heartbeat verificada en `cluster.Service.sendHeartbeat`.
- [x] Import: `Exists` en clientes compara ahora el conjunto exacto de grupos asignados (`sameGroupSet`) y los bloques repetidos de `piholeImportApply` se unificaron con `applyItem`.
- [x] Config: `settings.Config` protegido con `RWMutex` y `DNSSnapshot()` para lecturas concurrentes sin data races en el hot path DNS y escrituras desde la API (`backend/settings/concurrency_test.go`).
- [x] TECH: Cobertura P0 de backend completada con tests unitarios en `backend/audit/service_test.go`, `backend/whitelist/service_test.go`, `backend/prefetch/service_test.go` y `backend/api/ratelimit/ratelimit_test.go`.
- [x] FEATURE: Bloqueo por tipo de consulta DNS (`qtype`, p. ej. `AAAA`, `HTTPS`, `SVCB`) por politica (`Policy.BlockedQueryTypes`) y global (`dns.blockedQueryTypes`), respondiendo NODATA con EDE `Filtered` y reflejado en `ExplainWithType` (`backend/policy/qtype_test.go`).
- [x] FEATURE: Modo declarativo GitOps-first (`api.readOnly` bloqueando mutaciones API con 403 salvo login/logout) + recarga de `settings.yaml` en caliente via `SIGHUP` (`ReloadFromDisk`/`ReloadFromFile` con preservacion de secretos; tests en `backend/api/readonly_test.go` y `backend/settings/reload_test.go`).
- [x] FEATURE: Resync logico en caliente para HA (`GET /api/native/cluster/snapshot` firmado con HMAC + `POST /api/ha/resync`) que sincroniza whitelist, resoluciones DNS locales y leases DHCP estaticos en la replica sin reiniciar el nodo (`backend/api/cluster_snapshot_test.go`).
- [x] FEATURE: Helm chart oficial para Kubernetes en `charts/goaway/` con ConfigMap declarativo (`api.readOnly: true`), PVC opcional, ejecucion non-root con `NET_BIND_SERVICE` y servicios DNS (UDP/TCP/DoQ) y Web/API.
- [x] FEATURE (EPIC-08): Consistencia de politicas dual-stack IPv4/IPv6 (`backend/group/dualstack.go`) correlacionando direcciones hermanas por MAC y hostname para aplicar los mismos grupos y bloqueos en IPv4 e IPv6 (`backend/group/dualstack_test.go`).
- [x] FEATURE (EPIC-05): Reloj logico Lamport por entidad en replicacion HA (`backend/cluster/versioning.go`, `ReplicatedEvent.Version`/`OriginID`) para descartar eventos obsoletos tras cambio de lider o split-brain (`backend/cluster/versioning_test.go`).
- [x] FEATURE (EPIC-09): Exportador externo no bloqueante de query logs (`logging.queryLogExport`: NDJSON, CSV con cabecera unica o `stdout`) integrado en `ProcessLogEntries` (`backend/dns/server/query_log_exporter.go` y `query_log_exporter_test.go`).
- [x] FEATURE (EPIC-05): Replicacion de grupos y asignaciones por nombre estable (`groupName`/`groupNames`) con fallback a ID, y soporte HTTPS/mTLS entre peers HA (`highAvailability.tls`, `backend/cluster/tls.go` y `tls_test.go`).
- [x] FEATURE (EPIC-09): Cache DNS compartida opcional en Redis (cliente RESP2 ligero sin dependencias externas en `backend/dns/server/redis_cache.go`, write-through + hidratacion en miss, test entre dos nodos en `redis_cache_test.go`).
- [x] FEATURE (EPIC-07): Login OIDC (`api.oidc`) con discovery `/.well-known/openid-configuration`, flujo authorization code + cookie `oidc_state`, intercambio de token, mapeo de claims a roles (`admin`/`viewer`), auto-aprovisionamiento y logout (`backend/api/oidc.go`, `oidc_test.go` y boton SSO en `client/src/pages/login.tsx`).
- [x] FEATURE (EPIC-07): Integracion LDAP/Active Directory (`api.ldap`) con LDAPS/StartTLS, escape anti-inyeccion de filtros, mapeo de grupos (`memberOf` DN o CN) a roles `admin`/`viewer` y fallback local de emergencia (`backend/api/ldap.go` y `ldap_test.go`).
- [x] FEATURE (EPIC-09): Sink SQL externo para PostgreSQL/TimescaleDB (`logging.queryLogExport.format: "postgres"|"timescaledb"`) con auto-creacion de tabla `goaway_query_logs` e insercion transaccional por lotes (`backend/dns/server/query_log_postgres.go` y `query_log_postgres_test.go`). EPIC-09 completada al 100%.
- [x] FEATURE (EPIC-07): 2FA TOTP por usuario (RFC 6238 en `backend/api/totp.go`, step-up al desactivar y exigencia en `/api/login` + UI) y WebAuthn/Passkeys (`backend/api/webauthn.go`, alta/baja de credenciales y registro/login; tests en `backend/api/totp_webauthn_test.go`). EPIC-07 completada al 100%.
- [x] FEATURE (EPIC-06): Provider oficial de Terraform en `terraform-provider-goaway/` (Terraform Plugin Framework) con recursos `goaway_upstream`, `goaway_resolution`, `goaway_whitelist_domain` y `goaway_group`, cliente tipado, ejemplos HCL y tests unitarios. EPIC-06 completada al 100%.
- [x] FEATURE (EPIC-11): Soporte DoH3 (DNS over HTTP/3, RFC 9114 + RFC 8484) en cliente upstream (`doh3://` / `h3://` con `quic-go/http3`, transporte compartido, ID=0 en wire y restauracion de ID en respuesta) y listener servidor opcional (`dns.ports.doh3`), cabecera `Alt-Svc` en DoH, integracion en ciclo de vida (`ServiceRegistry`, `lifecycle.Manager`, `Application`) y tests E2E en `backend/dns/server/doh3_test.go`.
- [x] FEATURE (EPIC-11): Dashboard oficial de Grafana (`docs/grafana/goaway-dashboard.json` + `docs/grafana/README.md`) cubriendo las 13 metricas Prometheus de GoAway (QPS, bloqueo, cache/stale/prefetch, DNSSEC, percentiles P50/P95/P99, victorias y latencia por upstream) + plantilla opcional `ServiceMonitor` (`charts/goaway/templates/servicemonitor.yaml`) y puerto `doh3` en el Helm chart.
- [x] FEATURE (EPIC-11): Perfil ligero `stateless` (`misc.stateless: true`, flag `--stateless`, env `GOAWAY_STATELESS=true`) con SQLite en memoria (`database.InitializeWithOptions(true)`, sin crear archivos en disco), `misc.dashboard: false`, `api.readOnly: true`, `misc.inAppUpdate: false`, tests en `backend/database/stateless_test.go` y `backend/setup/stateless_test.go`, y perfil Helm multi-replica `charts/goaway/values-stateless.yaml`.
- [x] TECH (EPIC-11): Cobertura Vitest de componentes clave. jsdom + Testing Library con mock de `fetch` sobre fixtures de la API (`client/src/test/`); tests de login (2FA/OIDC), settings (guardado, password, export BD, DHCP), blacklist/whitelist y detalle de listas, query log y detalle de cliente, importador Pi-hole, resoluciones, prefetch, acciones de cabecera, `util.ts` y render de todas las rutas dentro del shell. 60.4% de lineas en `client/src/`; CI ejecuta `pnpm test --coverage` con umbral de 60%.
- [x] RELEASE (EPIC-11): Preparacion de v1.0.0. Guia de migracion 0.9x -> 1.0.0 (`docs/docs/migration-1.0.md`: `schemaVersion`, migraciones de BD, distribucion, rollback), entrada 1.0.0 en CHANGELOG y versiones a 1.0.0 (chart, client, compose, provider Terraform). Corregido el tag de imagen del chart (`v0.96.0` no existia; ahora por defecto `appVersion`).
- [x] SECURITY: Rotacion automatica del `jwtSecret` filtrado. Al arrancar se compara el SHA-256 del secreto con el filtrado en `settings.yaml` (6d91f07); si coincide se genera uno nuevo y se avisa en el log (`backend/setup/setup.go`). Para rotar a mano basta con dejar `api.jwtSecret` vacio.
- [x] BUG: Tamano de BD en modo stateless. `getDBSizeMB` usa `page_count * page_size` de SQLite con `misc.stateless: true`; corregida tambien la comprobacion del fichero principal, que nunca coincidia.
- [x] TECH (EPIC-11): Benchmarks y soak tests del resolver. `make bench-transport` (UDP, TCP, DoT, DoH, DoQ, DoH3 en proceso), `goleak.VerifyNone` en el arranque/parada de cada listener (CI) y `make soak` opcional contra un binario real. Baseline en `docs/SOAK.md`: 1000 QPS, 10 min por protocolo, 0 errores, RSS <= +0.5% tras calentamiento y goroutines de vuelta al reposo. Encontrados y corregidos: SERVFAIL en todos los aciertos de cache (`CodeToString` -> `RcodeToString`), doble prefijo de longitud en respuestas DoQ, conexiones DoQ bloqueadas tras ~100 consultas concurrentes y `/metrics` con doble gzip.

## EPIC-12 CI/CD GitHub (completada 2026-11-25)
- [x] DECISION: releases manuales por tag `v*` (proceso documentado en `CONTRIBUTING.md`); eliminados semantic-release y `.releaserc.json`.
- [x] CI: `pull-request.yml` sustituido por `.github/workflows/ci.yml` (push a `main`, PR y `workflow_call`) con jobs paralelos `frontend` (commitlint solo en PR, ESLint, Vitest, build, audit), `backend` (golangci-lint v2.15.0, govulncheck v1.10.0, `go test -race`), `e2e`, `helm` (lint + template default/stateless) y `terraform-provider` (vet, test, build); caches de Go y pnpm, `concurrency` y `timeout-minutes` en todos los jobs.
- [x] CI: `release.yml` disparado por tag, reutiliza `ci.yml` como `needs`, verifica que `Chart.yaml` `version`/`appVersion` coinciden con el tag, GoReleaser via `goreleaser/goreleaser-action` (v2.20.3) y push multi-arch a GHCR con `docker/login-action` + `GITHUB_TOKEN` (`packages: write`). Eliminados `curl git.io/goreleaser | bash` y el login a DockerHub.
- [x] CI: versiones fijadas (pnpm 10.23.0, golangci-lint, govulncheck, goreleaser, helm) y actions en su ultima major (`checkout@v7`, `setup-go@v7`, `setup-node@v7`, `setup-python@v7`, `cache@v6`, ...); `permissions` minimas por job. golangci-lint usa la config por defecto de v2 (sin `.golangci.yml`).
- [x] CI: `.github/dependabot.yml` semanal para gomod (raiz y provider), npm (`client/`), docker y github-actions con grupos minor/patch.
- [x] FIX: backend limpio con golangci-lint v2.15.0 (0 issues): 30 `Close`/`Write` sin comprobar, `PreferServerCipherSuites` obsoleto en DoH, simplificaciones staticcheck y bug en DHCPv4 donde los errores de `handleRelease`/`handleDecline` se descartaban sin log (`backend/dhcp/service.go`).
- [x] FIX: E2E con imagen `golang:1.26.5-alpine` (igual que la toolchain de `go.mod`, evita descarga en cada run) y timeout de arranque 180 s.
- [x] DEPS (EPIC-11): Go 1.26.5 -> 1.27.4 (`go 1.27` + `toolchain go1.27.4` en `go.mod` raiz y del provider, imagenes `golang:1.27.4-alpine` en `Dockerfile.dev`, `Dockerfile.multi` y `docker-compose.e2e.yml`). Corrige GO-2026-6859, GO-2026-6809, GO-2026-6617 y GO-2026-6613 de la stdlib detectadas por govulncheck en el primer run de CI.
- [x] OPS (EPIC-12): Actions verificado en `rasty94/goaway` (primer run de `CI` en verde en `6234490`), branch protection en `main` con checks obligatorios `frontend`, `backend`, `e2e`, `helm` y `terraform-provider` (sin force-push ni borrado; admins exentos con `enforce_admins: false`) y `can_approve_pull_request_reviews: false` para `GITHUB_TOKEN`.
- [x] DEPS (EPIC-11): Actualizacion de dependencias para v1.0.0 (2026-11-25):
  - Go: 11 modulos directos al dia (`miekg/dns` v0.6.143, `quic-go` v0.66.0, `minio-go` v7.4.1, `client_golang` v1.25.0, ...); `dnsutil.SetReply` ahora muta en sitio, la respuesta de rate-limit copia el mensaje (`ratelimit_reply_test.go`).
  - Toolchain/imagenes: Node 26 LTS, Alpine 3.24, pnpm 10.23.0 fijado en `Dockerfile.multi` (pnpm 11 rechazaba el lockfile). La imagen ahora incrusta la version (`--build-arg VERSION`).
  - Frontend minor/patch al dia; `react-hooks/set-state-in-effect` como warning, `useIsMobile` con `useSyncExternalStore`.
  - Majors frontend: commitlint 21 (declarado `conventional-changelog-conventionalcommits`), `@tanstack/react-table` 9 (tabla de logs con `useTable` y features explicitas, `table.test.tsx`), i18next 27 + react-i18next 18 (`i18n.test.ts`), motion 14, tsparticles 4 (`ParticlesProvider`, verificado en navegador). Pendiente solo TypeScript 7 (ver TODO.md).

