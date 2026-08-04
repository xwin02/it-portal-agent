// Package contracts contains extension points reserved for future sprints.
package contracts

import "context"

type Heartbeat interface { Send(context.Context) error }
type InventoryCollector interface { Collect(context.Context) error }
type SoftwareScanner interface { Scan(context.Context) error }
type CommandProcessor interface { Process(context.Context) error }
type Updater interface { Check(context.Context) error }
