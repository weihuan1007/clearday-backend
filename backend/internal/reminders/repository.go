package reminders

import "context"

type Repository interface {
	List(context.Context) ([]Reminder, error)
	Get(context.Context, string) (Reminder, error)
	Create(context.Context, Reminder) (Reminder, error)
	Update(context.Context, Reminder) (Reminder, error)
	Delete(context.Context, string) error
}
