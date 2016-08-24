package proto

// Account struct
type Account struct {
	URL        string
	UUID       string
	Login      string
	Title      string
	FirstName  string `json:"first_name"`
	MiddleName string `json:"middle_name"`
	LastName   string `json:"last_name"`
	Email      struct {
		Email    string
		IsPublic bool `json:"is_public"`
	}
	Affiliation struct {
		Institute  string
		Department string
		City       string
		Country    string
		IsPublic   bool `json:"is_public"`
	}
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// Query struct
type Query struct {
	q string
}

// AuthError authentication error struct
type AuthError struct {
	Code    int    `json:"code"`
	Error   string `json:"error"`
	Message string `json:"message"`
	Reasons struct {
		GrantType string `json:"grant_type"`
	}
}

// AuthResponse authentication response
type AuthResponse struct {
	Scope       string `json:"scope"`
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
}

// SSHKey SSH key information struct
type SSHKey struct {
	URL         string `json:"url"`
	Fingerprint string
	Key         string
	Description string
	Login       string
	AccountURL  string `json:"account_url"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}
