# GoAway - Tareas Completadas

Ultima actualizacion: 2026-11-24

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
