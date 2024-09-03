package newauth

type LDAPAuthResult struct {
	username string
	password string
	orgs     []string
}

var _ AuthResult = (*LDAPAuthResult)(nil)

func (lr *LDAPAuthResult) GetUser() string {
	return lr.username
}

func (lr *LDAPAuthResult) GetOrgs() []string {
	return lr.orgs
}

func (lr *LDAPAuthResult) auth() {
	// do the auth logic
	lr.orgs = []string{}
}

func LDAPAuth(username, password string) AuthResult {
	lr := &LDAPAuthResult{
		username: username,
	}
	lr.auth()
	return lr
}
