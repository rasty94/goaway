# GoAway - Hoja de Ruta y TODO

Ultima actualizacion: 2026-11-24

Este archivo solo contiene trabajo pendiente. Lo completado esta en [DONE.md](DONE.md).

Tablero recomendado (GitHub Projects): Inbox, Ready, In Progress, Review, Blocked, Done.

Labels base: epic, feature, enhancement, security, dns, dhcp, ha, frontend, api, observability, migration, breaking-change, good-first-issue, p0, p1, p2.

---

## Epicas abiertas

| Epic | Objetivo | Prioridad | Estado |
| --- | --- | ---: | --- |
| EPIC-05 HA Active | Versionado para conflictos tras split-brain | P2 | Auth, outbox y cobertura hechos |
| EPIC-06 Ecosystem | Helm/Terraform y modo declarativo GitOps | P1 | API keys e importador Pi-hole hechos |
| EPIC-07 Enterprise Auth | OIDC, LDAP/AD, passkeys, RBAC fino | P2 | Sin empezar |
| EPIC-08 DHCPv6 y RA | Consistencia de politicas dual-stack | P2 | DHCPv6+RA hechos |
| EPIC-09 Resolver Avanzado | DoQ, parallel_best, EDE, Redis/logs externos | P1 | Sin empezar |

---

## P0 - Deuda que bloquea lo demas

- [ ] TECH: Ampliar cobertura de tests en backend
  - Prioridad: p0
  - Labels: enhancement
  - Contexto: 13 ficheros `_test.go`. Sin tests en `cluster`, `dhcp`, `dns` (fuera de server), `audit`, `user`, `whitelist`, `prefetch`, `resolution`.
  - Criterios de aceptacion:
    - Tests en `cluster` (replicacion, election) y `dhcp` (v4/v6 handlers).
    - Tests en el cliente React (hoy no hay script `test` en package.json).

- [ ] TECH: Runner de tests en el cliente
  - Prioridad: p0
  - Labels: frontend, enhancement
  - Criterios de aceptacion:
    - Script `test` en `client/package.json` y ejecucion en CI.

---

## Seguimiento de la revision del 2026-11-24

Hallazgos resueltos (ver [DONE.md](DONE.md)).

---

## Seguimiento de la revision del 2026-09-07

Hallazgos verificados por las revisiones de codigo y seguridad que no se corrigieron en `3b10071`.

- [ ] HA: resync automatico cuando un peer pierde eventos. Bloqueado por diseno: `ImportTeleporterData` reemplaza el fichero de base de datos entero y exige reinicio, asi que no vale como resync en caliente. Hace falta un snapshot logico (listas, grupos, clientes, DNS local, leases) aplicable sin reiniciar, o un endpoint de re-seed que reinicie el nodo de forma controlada. Mientras tanto la perdida ya no es silenciosa: el nodo se marca "Out of sync" en el dashboard.
- [ ] HA: peers por HTTPS o firma de la respuesta de heartbeat. Un MITM en la red de peers puede forzar la eleccion.
- [ ] HA: cache de nonces en la ventana de 30 s (hoy un evento capturado se puede reenviar dentro de la ventana; efecto idempotente).
- [ ] Import: `Exists` en clientes se calcula como "tiene alguna asignacion"; deberia comparar el conjunto de grupos.
- [ ] Config: `settings.Config` se lee desde el resolver y se escribe desde la API sin mutex (upstreams, DHCP, cache). Hoy se mitiga con copy-on-write en upstreams; hace falta un puntero atomico o RWMutex en el hot path.
- [ ] Simplificaciones pendientes de ponytail-review: los nueve bloques `if !Exists` en `piholeImportApply` siguen repitiendo el mismo esqueleto.

## P1 - Operacion, Resiliencia y Ecosistema

### EPIC-05 HA Active

- [ ] FEATURE: Resolucion de conflictos con versionado
  - Prioridad: p2
  - Labels: feature, ha
  - Contexto: hoy el Primary es la unica fuente de escritura y el outbox entrega en orden, asi que no hay conflictos salvo split-brain. Los eventos de grupo viajan por ID autoincremental, por lo que ambos nodos deben partir del mismo snapshot (backup Fase 1).
  - Criterios de aceptacion:
    - Version o reloj logico por entidad; el replica descarta eventos mas antiguos que su estado.
    - Grupos referenciados por nombre o UUID estable en vez de ID autoincremental.
    - Test de reconciliacion tras un cambio de lider.

### EPIC-06 Ecosystem

- [ ] FEATURE: Helm chart + provider Terraform oficial
  - Prioridad: p1
  - Labels: feature, api
  - Criterios de aceptacion:
    - Instalacion reproducible en Kubernetes.
    - Recursos de provider para upstreams, listas, grupos, politicas.
    - Ejemplos verificados en CI.

- [ ] FEATURE: Modo declarativo / GitOps-first opcional
  - Prioridad: p2
  - Labels: feature, api, enhancement
  - Contexto: complemento del Helm chart para entornos Kubernetes/homelab declarativos (inspirado en Blocky), donde `settings.yaml` y listas/reglas se gestionan desde Git sin estado mutable en UI.
  - Criterios de aceptacion:
    - Flag/opcion de configuracion de solo lectura para bloquear mutaciones desde API/UI.
    - Carga y recarga declarativa (SIGHUP o watcher) de upstreams, listas, grupos y resoluciones locales.

### EPIC-09 Resolver Avanzado (Paridad DNS / Blocky)

- [ ] FEATURE: Bloqueo por tipo de consulta DNS (qtype)
  - Prioridad: p2
  - Labels: feature, dns
  - Criterios de aceptacion:
    - Regla por grupo o cliente para bloquear/responder NODATA a tipos concretos (p. ej. `AAAA`, `HTTPS`, `SVCB`).
    - Reflejado en el simulador de politicas y endpoint de explicacion DNS.

- [ ] FEATURE: Cache compartida opcional en Redis y exportacion externa de query logs
  - Prioridad: p2
  - Labels: feature, ha, observability
  - Contexto: permite escalar despliegues multi-instancia reduciendo carga sobre SQLite en redes grandes.
  - Criterios de aceptacion:
    - Backend de cache opcional en Redis (write-through) compartiendo entradas y estado de pausa entre nodos.
    - Exportador configurable de query logs a PostgreSQL/TimescaleDB o salida estructurada (JSON/CSV) sin bloquear el hot path DNS.

---

## P2 - Enterprise y Dual Stack Completo

### EPIC-07 Enterprise Auth

- [ ] FEATURE: Login OIDC
  - Prioridad: p2
  - Labels: feature, security, api
  - Criterios de aceptacion:
    - Login con proveedores OIDC comunes.
    - Mapeo de claims a roles.
    - Cierre de sesion y expiracion.

- [ ] FEATURE: Integracion LDAP/AD
  - Prioridad: p2
  - Labels: feature, security, api
  - Criterios de aceptacion:
    - Bind seguro y mapeo de grupos.
    - Fallback local de emergencia.

- [ ] FEATURE: WebAuthn/passkeys
  - Prioridad: p2
  - Labels: feature, security, frontend
  - Criterios de aceptacion:
    - Alta/baja de credenciales.
    - Step-up para acciones sensibles.
    - 2FA por usuario.

### EPIC-08 DHCPv6 y Router Advertisements

- [ ] FEATURE: Consistencia de politicas dual-stack
  - Prioridad: p2
  - Labels: feature, dhcp, dns
  - Criterios de aceptacion:
    - Misma politica para IPv4/IPv6 del mismo cliente.
    - Correlacion automatica de identidad dual-stack.

---

## Milestones

- [x] v0.9 Resolver: EPIC-01 + base de EPIC-03.
- [x] v1.0 Policy: EPIC-02 + resto de EPIC-03 + privacidad base.
- [x] v1.1 Cluster: EPIC-05 (auth, outbox persistente), importador Pi-hole y API keys con scopes.
- [ ] v1.2 Enterprise: auth avanzada + consistencia dual-stack + ecosistema IaC/SDK.

Nota: el CHANGELOG va por 0.93.0 mientras que v0.9 y v1.0 ya estan cubiertos funcionalmente. Decidir si se salta a 1.0.0 en la proxima release.

---

## Definition of Done (Transversal)

- Tests unitarios.
- Tests de integracion o E2E.
- Metricas y logs necesarios.
- Documentacion actualizada.
- Changelog actualizado.
- Plan de migracion y rollback.
- Mover la tarea de este fichero a DONE.md.

## Plantilla Minima de Issue

Goal, User story, Scope, Out of scope, API/Config changes, UI changes, Metrics/Logs, Security impact, Migration/Upgrade impact, Acceptance criteria, Test plan.
