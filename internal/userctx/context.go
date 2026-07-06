package userctx

import "context"

type contextKey string

const userContextKey contextKey = "scriptagent_user"

type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
}

func WithUser(ctx context.Context, user User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

func FromContext(ctx context.Context) (User, bool) {
	user, ok := ctx.Value(userContextKey).(User)
	return user, ok
}

func UserID(ctx context.Context) string {
	user, ok := FromContext(ctx)
	if !ok {
		return ""
	}
	return user.ID
}
