package newauth

type AuthResult interface {
	GetUser() string
	GetOrgs() []string
}
