# Architecture Glossary

This glossary defines the canonical architecture vocabulary used across `uptime-lab`. It is intentionally limited to cross-runtime and system-design concepts; framework-specific terms belong in later area documentation.

## Control Plane

The Go runtime that owns product/domain coordination, public and internal API semantics, due-work coordination, and durable product state.

## Execution Plane

The Rust runtime that owns bounded probe execution against external targets. It does not own primary product persistence or product policy.

## Web Client

The React/TypeScript browser application that owns presentation and user interaction. It consumes the public contract and never accesses PostgreSQL directly.

## Monitor

A conceptual configured target plus product-owned monitoring policy. The Monitoring capability owns its lifecycle and product semantics.

## Probe

One bounded execution attempt against a target through a protocol adapter.

## Check Result

A normalized execution result submitted by the Execution Plane to the Control Plane for product interpretation and durable-state updates.

## Module

An independently owned business capability inside the Go modular monolith, with its own domain/application boundary and persistence responsibility.

## Port

An inward-facing abstraction that defines an external capability required by core/application logic without depending on a concrete infrastructure implementation.

## Adapter

A transport or infrastructure implementation that connects a port or runtime boundary to an external technology or system.

## Domain Event

An in-process fact representing a completed domain occurrence. Domain events do not automatically imply asynchronous delivery or a broker.

## Integration Event

A cross-process/domain fact introduced only when reliable delivery across a process or ownership boundary becomes a concrete requirement.

## Public Contract

The API contract consumed by the Web Client and, later, potentially external clients. It is separate from storage representations and internal checker communication.

## Internal Contract

The controlled runtime-to-runtime contract used by the Go Control Plane and Rust Execution Plane for work descriptions and normalized results.

## Architecture Fitness Function

An automated check that detects a forbidden architectural dependency or invariant violation.

## Canonical Documentation

Repository-owned documentation that is authoritative for a specific architecture boundary, ownership rule, flow, or decision.
