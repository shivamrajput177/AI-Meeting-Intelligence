// Package domain holds Organization Service's entities and the
// repository interface its usecases depend on — no external deps here,
// per the Clean Architecture layering in
// docs/architecture/folder-structure.md.
package domain

import "time"

type Organization struct {
	ID        string
	Name      string
	Slug      string
	Plan      string
	Status    string
	CreatedAt time.Time
}
