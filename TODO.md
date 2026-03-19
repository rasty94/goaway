# GoAway - Hoja de Ruta y TODO

Ultima actualizacion: 2026-03-19

## Resumen Ejecutivo

Este archivo consolida:
- Estado real de lo ya implementado.
- Pendientes tecnicos de corto y medio plazo.
- Backlog accionable para GitHub Projects (epicas, prioridades, labels y criterios).

Tablero recomendado (GitHub Projects):
- Inbox
- Ready
- In Progress
- Review
- Blocked
- Done

Labels base recomendadas:
- epic
- feature
- enhancement
- security
- dns
- dhcp
- ha
- frontend
- api
- observability
- migration
- breaking-change
- good-first-issue
- p0
- p1
- p2

---

## Completado

### DX, CI/CD y Operacion
- [x] Cobertura y linters en CI (incluyendo golangci-lint).
- [x] Firma/checksums en release.
- [x] Especificacion OpenAPI/Swagger.
- [x] Guia para ejecutar sin root en puerto 53.
- [x] Apagado graceful (SIGINT/SIGTERM) en ciclo de vida.

### Frontend y UX
- [x] Localizacion completa (EN/ES).
- [x] Cola de consultas en vivo (WebSocket multi-cliente).
- [x] Visualizaciones avanzadas de actividad.
- [x] Grafo de topologia de clientes.
- [x] Endurecimiento mobile-first en rutas clave.
- [x] Dashboard HTMX alpha sin dependencia Node para ese modo.

### DNS y Politicas Base
- [x] Capa de cache DNS con TTL y control desde UI.
- [x] DNS local y registros CNAME/A/AAAA.
- [x] Allowlist completa.
- [x] Wildcards en allow/block.
- [x] Bloqueo por regex.
- [x] Reenvio condicional por dominio.

### Arquitectura, Backup y Observabilidad
- [x] Export/Import tipo Teleporter (settings + base de datos).
- [x] Backups remotos (S3, WebDAV, local montado).
- [x] Metricas Prometheus en /metrics.

### Seguridad y Control de Acceso
- [x] Multiusuario para administracion.
- [x] Rate limiting por cliente con metrica dedicada.
- [x] Gestion de grupos por cliente (IP/MAC) con enforcement en resolucion.

### Red y Plataforma
- [x] Servidor DHCPv4 nativo.
- [x] Gestion web de DHCP.
- [x] Leases estaticos DHCP.
- [x] HA Fase 1: sincronizacion pasiva Primary -> Replica por backup + teleporter (manual y programada).
- [x] Smoke E2E en Docker para validacion de endpoints criticos.

---

## Pendientes Activos (Roadmap Tecnico)

### Pendientes de Alta Prioridad Tecnica
- [x] Framework de migraciones de esquema (versionado + rollback).
- [ ] DNSSEC completo con estado secure/insecure/bogus en logs/API.
- [ ] Soporte total Windows/macOS (de Beta a Full).
- [ ] DHCPv6 nativo.

---

## Backlog Accionable para GitHub Projects

## Epicas

| Epic | Objetivo | Prioridad | Dependencias |
|---|---|---:|---|
| EPIC-01 Secure Resolver | DoT/DoH/DoQ, DNSSEC, failover de upstream, serve-stale | P0 | Cache + runtime config |
| EPIC-02 Policy Engine | Politicas por cliente/grupo/subred/horario/categoria | P0 | Grupos + allow/block + regex |
| EPIC-03 Explainability | Explicar decisiones DNS, simulador, dry-run | P0 | Policy Engine |
| EPIC-04 Privacy & Audit | Retencion, anonimizado, auditoria administrativa | P1 | Multiusuario |
| EPIC-05 HA Active | Active-active, election, replicacion en tiempo real, VIP | P1 | HA Fase 1 |
| EPIC-06 Ecosystem | Importador Pi-hole, API keys con scopes, Helm/Terraform | P1 | API estable |
| EPIC-07 Enterprise Auth | OIDC, LDAP/AD, passkeys, RBAC fino | P2 | Multiusuario |
| EPIC-08 DHCPv6 y RA | DHCPv6 + Router Advertisements + dual-stack consistente | P2 | DHCPv4 |

---

## P0 - Siguiente Milestone

### EPIC-01 Secure Resolver

- [ ] FEATURE: Soporte de upstream DoT saliente
  - Prioridad: p0
  - Labels: epic, feature, dns, security
  - Criterios de aceptacion:
    - Multiples upstream DoT con SNI y validacion de certificado.
    - Health checks periodicos y salida del pool en fallo.
    - Metricas por upstream: latencia, timeout, errores TLS.

- [ ] FEATURE: Soporte de upstream DoH/DoQ saliente
  - Prioridad: p0
  - Labels: feature, dns, security
  - Criterios de aceptacion:
    - DoH sobre HTTP/2 y DoQ sobre QUIC.
    - Bootstrap sin dependencia circular.
    - Fallback ordenado entre UDP/TCP/DoT/DoH/DoQ.

- [ ] FEATURE: DNSSEC completo
  - Prioridad: p0
  - Labels: feature, dns, security
  - Criterios de aceptacion:
    - Validacion de cadena de confianza.
    - Estado secure/insecure/bogus en logs y API.
    - Diagnostico por dominio con causa de fallo.

- [ ] FEATURE: Cache serve-stale + prefetch inteligente
  - Prioridad: p0
  - Labels: feature, dns, enhancement
  - Criterios de aceptacion:
    - Responder con cache expirada cuando upstreams fallen.
    - Prefetch de entradas calientes antes de expirar.
    - Metricas separadas para hit, stale-hit y prefetch-hit.

### EPIC-02 Policy Engine

- [ ] FEATURE: Motor jerarquico de politicas
  - Prioridad: p0
  - Labels: feature, dns, api
  - Criterios de aceptacion:
    - Precedencia explicita: global > grupo > subred > cliente.
    - Condiciones por IP, MAC, CIDR, hostname, dia y franja horaria.
    - API devuelve regla ganadora y origen.

- [ ] FEATURE: Filtrado por categorias
  - Prioridad: p0
  - Labels: feature, dns, frontend
  - Criterios de aceptacion:
    - Categorias iniciales: ads, trackers, malware, adult, gambling, social.
    - Activacion por grupo y por horario.
    - Conteos por categoria y fuente.

- [ ] FEATURE: SafeSearch y modos restringidos
  - Prioridad: p0
  - Labels: feature, dns
  - Criterios de aceptacion:
    - Forzado en motores principales por grupo.
    - Excepciones por cliente/subred.
    - Resultado visible en simulador.

### EPIC-03 Explainability

- [ ] FEATURE: Endpoint de explicacion DNS
  - Prioridad: p0
  - Labels: feature, api, dns
  - Criterios de aceptacion:
    - Endpoint de explicacion con fqdn, tipo, cliente y timestamp.
    - Devuelve matching (listas, wildcard, regex), upstream y TTL final.
    - Indica cache-hit o stale-hit.

- [ ] FEATURE: Simulador de politicas en UI
  - Prioridad: p0
  - Labels: feature, frontend
  - Criterios de aceptacion:
    - Formulario dominio + cliente + fecha/hora.
    - Arbol de evaluacion y regla final.
    - Comparativa actual vs draft.

- [ ] FEATURE: Dry-run para nuevas reglas
  - Prioridad: p0
  - Labels: feature, dns, observability
  - Criterios de aceptacion:
    - Dry-run no bloquea, solo registra impacto.
    - Panel con posibles falsos positivos.
    - Exportacion CSV para revision.

---

## P1 - Operacion, Resiliencia y Ecosistema

### EPIC-04 Privacy & Audit

- [ ] FEATURE: Retencion por capas
  - Prioridad: p1
  - Labels: feature, observability
  - Criterios de aceptacion:
    - Retencion separada para raw logs, agregados y metricas.
    - Purga automatica visible en UI.
    - Configuracion por tipo de dato.

- [ ] FEATURE: Anonimizacion IP
  - Prioridad: p1
  - Labels: feature, security
  - Criterios de aceptacion:
    - Truncado IPv4/IPv6 en almacenamiento/export.
    - Modo reversible solo con clave activa.
    - Aviso de impacto en troubleshooting.

- [ ] FEATURE: Auditoria administrativa
  - Prioridad: p1
  - Labels: feature, security, observability
  - Criterios de aceptacion:
    - Registra login, cambios de config, import/restore y reglas.
    - Evento con actor, timestamp, IP origen y diff logico.
    - Export JSON y webhook opcional.

### EPIC-05 HA Active

- [ ] FEATURE: Membership y leader election
  - Prioridad: p1
  - Labels: feature, ha
  - Criterios de aceptacion:
    - Roles leader/follower/standby.
    - Heartbeats y fencing basico anti split-brain.
    - Estado visible en dashboard y metricas.

- [ ] FEATURE: Replicacion en tiempo real
  - Prioridad: p1
  - Labels: feature, ha
  - Criterios de aceptacion:
    - Replica listas, grupos, clientes, leases y DNS local en segundos.
    - Cola persistente y reintentos al reconectar nodos.
    - Resolucion de conflictos con versionado definido.

- [ ] FEATURE: Modo despliegue con VIP flotante
  - Prioridad: p1
  - Labels: feature, ha
  - Criterios de aceptacion:
    - Guia oficial de despliegue con VIP compartida.
    - Health checks expulsan nodo en fallo DNS/DB.
    - Runbook de failover y rollback.

### EPIC-06 Ecosystem

- [ ] FEATURE: Wizard de importacion Pi-hole
  - Prioridad: p1
  - Labels: feature, api, frontend
  - Criterios de aceptacion:
    - Importa listas, grupos, clientes, DNS local, regex y settings.
    - Vista previa con conflictos antes de aplicar.
    - Reporte final de migracion.

- [ ] FEATURE: API keys con scopes
  - Prioridad: p1
  - Labels: feature, api, security
  - Criterios de aceptacion:
    - Scopes: read, write, backup, metrics, admin.
    - Expiracion, revocacion y nombre descriptivo.
    - Ultimo acceso y logs de uso.

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

- [ ] FEATURE: DHCPv6 nativo
  - Prioridad: p2
  - Labels: feature, dhcp
  - Criterios de aceptacion:
    - Scopes, reservas, opciones base y estado de leases.

- [ ] FEATURE: Gestion de Router Advertisements
  - Prioridad: p2
  - Labels: feature, dhcp
  - Criterios de aceptacion:
    - Modos SLAAC, stateless y managed.
    - Validacion de prefijos y timers.

- [ ] FEATURE: Consistencia de politicas dual-stack
  - Prioridad: p2
  - Labels: feature, dhcp, dns
  - Criterios de aceptacion:
    - Misma politica para IPv4/IPv6 del mismo cliente.
    - Correlacion automatica de identidad dual-stack.

---

## Foundations Tecnicas Transversales

- [ ] TECH: Framework de migraciones de esquema
- [x] TECH: Framework de migraciones de esquema
  - Prioridad: p0
  - Labels: migration, breaking-change
  - Criterios de aceptacion:
    - Migraciones versionadas con up/down.
    - Runner idempotente en startup.
    - Estado y rollback por pasos implementados.

- [x] TECH: Esquema versionado de configuracion + upgrade hooks
  - Prioridad: p0
  - Labels: migration, breaking-change
  - Criterios de aceptacion:
    - Campo de version de configuracion.
    - Hooks de upgrade/validacion al cargar settings.
    - Persistencia automatica de upgrades compatibles.

---

## Definition of Done (Transversal)

- [ ] Tests unitarios.
- [ ] Tests de integracion o E2E.
- [ ] Metricas y logs necesarios.
- [ ] Documentacion actualizada.
- [ ] Changelog actualizado.
- [ ] Plan de migracion y rollback.

---

## Milestones Propuestos

- [ ] v0.9 Resolver: EPIC-01 + base de EPIC-03.
- [ ] v1.0 Policy: EPIC-02 + resto de EPIC-03 + privacidad base.
- [ ] v1.1 Cluster: EPIC-05 + importador Pi-hole + API keys con scopes.
- [ ] v1.2 Enterprise: auth avanzada + DHCPv6 + ecosistema IaC/SDK.

---

## Plantilla Minima de Issue

- [ ] Goal
- [ ] User story
- [ ] Scope
- [ ] Out of scope
- [ ] API/Config changes
- [ ] UI changes
- [ ] Metrics/Logs
- [ ] Security impact
- [ ] Migration/Upgrade impact
- [ ] Acceptance criteria
- [ ] Test plan
