# Status codes

This lists the possible status codes that can be returned by each API endpoint on the GIN server.
We only list the endpoints that the GIN Client uses.

All endpoints may return 500 in case of general server error.

All endpoints may return the following:

- 401 Unauthorized: Invalid credentials (in case of login) OR Invalid or no token.
- 500 Internal Server Error: Any server error


## POST `/api/v1/users/<user>/tokens`

**Description**: Request a new user token (login).

### Status codes
- 201 Created: New access token created.


## POST `/api/v1/user/repos`

**Description**: Create new user repository.

### Status codes
- 201 Created: New repository created.
- 422 Unprocessable Entity: Repository already exists OR Repository name is reserved OR Repository name is invalid.

### Notes
Extra condition that returns:
- 422 Unprocessable Entity: Cannot create repository for organisation.

Shouldn't happen. Failsafe for bad routing? Will ignore in client.


## GET `/api/v1/repos/<user>/<repository>`

**Description**: Retrieve information about a specific repository.

### Status codes
- 200 OK: Success.
- 404 Not Found: User or Repository (path) does not exist.


## GET `/api/v1/user/repos`

**Description**: Retrieve a list of user repositories.

### Status codes
- 200 OK: Success.


## DELETE `/api/v1/repos/<user>/<repository>`

**Description**: Delete a repository.

### Status codes
- 204 No Content: Success.
- 403 Forbidden: User is not owner or admin of repository.
- 404 Not Found: User or Repository (path) does not exist.


## GET `/api/v1/user/keys`

**Description**: List user keys.

### Status codes
- 200 OK: Success.


## POST `/api/v1/user/keys`

**Description**: Add a new user key.

### Status codes
- 201 Created: New user key added.
- 422 Unprocessable Entity: Invalid key or duplicate ID/name.


## DELETE `/api/v1/user/keys/<keyid>`

**Description**: Delete a user key.

### Status codes
- 204 No Content: Success.
- 403 Forbidden: User does not have access to key.


## GET `/api/v1/users/<user>`

**Description**: Retrieve information about a specific user account.

### Status codes
- 200 OK: Success.
- 404 Not Found: User does not exist.
