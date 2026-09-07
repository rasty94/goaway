# GoAway - Hoja de Ruta y TODO

Ultima actualizacion: 2026-09-07

Este archivo solo contiene trabajo pendiente. Lo completado esta en [DONE.md](DONE.md).

Tablero recomendado (GitHub Projects): Inbox, Ready, In Progress, Review, Blocked, Done.

Labels base: epic, feature, enhancement, security, dns, dhcp, ha, frontend, api, observability, migration, breaking-change, good-first-issue, p0, p1, p2.

---

## Epicas abiertas

| Epic | Objetivo | Prioridad | Estado |
| --- | --- | ---: | --- |
| EPIC-05 HA Active | Versionado para conflictos tras split-brain | P2 | Auth, outbox y cobertura hechos |
| EPIC-06 Ecosystem | Helm/Terraform | P1 | API keys e importador Pi-hole hechos |
| EPIC-07 Enterprise Auth | OIDC, LDAP/AD, passkeys, RBAC fino | P2 | Sin empezar |
| EPIC-08 DHCPv6 y RA | Consistencia de politicas dual-stack | P2 | DHCPv6+RA hechos |

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
