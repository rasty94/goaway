# GoAway - Hoja de Ruta y TODO

Ultima actualizacion: 2026-11-24

Este archivo solo contiene trabajo pendiente. Lo completado esta en [DONE.md](DONE.md).

Tablero recomendado (GitHub Projects): Inbox, Ready, In Progress, Review, Blocked, Done.

Labels base: epic, feature, enhancement, security, dns, dhcp, ha, frontend, api, observability, migration, breaking-change, good-first-issue, p0, p1, p2.

---

## Epicas abiertas

| Epic | Objetivo | Prioridad | Estado |
| --- | --- | ---: | --- |
| EPIC-05 HA Active | Grupos por UUID/nombre estable y TLS mutuo | P2 | Auth, outbox, resync y reloj logico hechos |
| EPIC-06 Ecosystem | Provider Terraform oficial | P1 | API keys, Pi-hole import, GitOps y Helm hechos |
| EPIC-07 Enterprise Auth | OIDC, LDAP/AD, passkeys, RBAC fino | P2 | Sin empezar |
| EPIC-09 Resolver Avanzado | Cache compartida Redis / sink SQL externo | P1 | DoQ, parallel_best, EDE, qtype y export JSON/CSV hechos |

---

## P0 - Deuda que bloquea lo demas

(Sin bloqueantes P0 pendientes; ver [DONE.md](DONE.md).)

---

## Seguimiento de la revision del 2026-11-24

Hallazgos resueltos (ver [DONE.md](DONE.md)).

---

## Seguimiento de la revision del 2026-09-07

Hallazgos verificados por las revisiones de codigo y seguridad que no se corrigieron en `3b10071`.

- [ ] HA: peers por HTTPS (TLS mutuo/certificados entre nodos; la firma HMAC de la respuesta de heartbeat y el resync logico en caliente ya estan implementados).

## P1 - Operacion, Resiliencia y Ecosistema

### EPIC-05 HA Active

- [ ] FEATURE: Grupos referenciados por nombre o UUID estable en replicacion HA
  - Prioridad: p2
  - Labels: feature, ha
  - Contexto: el reloj logico Lamport (`cluster.VersionTracker`) ya descarta eventos obsoletos tras cambio de lider; queda desacoplar eventos de grupo del ID autoincremental.
  - Criterios de aceptacion:
    - Grupos referenciados por nombre o UUID estable en vez de ID autoincremental.

### EPIC-06 Ecosystem

- [ ] FEATURE: Provider Terraform oficial
  - Prioridad: p1
  - Labels: feature, api
  - Criterios de aceptacion:
    - Recursos de provider para upstreams, listas, grupos, politicas.
    - Ejemplos verificados en CI.

### EPIC-09 Resolver Avanzado (Paridad DNS / Blocky)

- [ ] FEATURE: Cache compartida opcional en Redis y sink SQL externo (PostgreSQL/TimescaleDB)
  - Prioridad: p2
  - Labels: feature, ha, observability
  - Contexto: el exportador estructurado JSON/CSV/stdout sin bloqueo ya esta implementado; queda la capa opcional Redis (write-through) y sink SQL para despliegues multi-instancia de gran escala.
  - Criterios de aceptacion:
    - Backend de cache opcional en Redis compartiendo entradas y estado de pausa entre nodos.
    - Exportador opcional a PostgreSQL/TimescaleDB.

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
