# Unreleased

-

# v2.7.0

- [added] Added several new functions for testing errors
  (e.g. `auth.IsUserNotFound()`).
- [added] Added support for setting the `mutable-content` property on
  FCM messages sent via APNS.
- [changed] Updated the error messages returned by the `messaging`
  package. These errors now contain the full details sent by the
  back-end server.

# v2.6.1

- [added] Added support for Go 1.6.
- [changed] Fixed a bug in the
  [`UnsubscribeFromTopic()`](https://godoc.org/firebase.google.com/go/messaging#Client.UnsubscribeFromTopic)
  function.
- [changed] Improved the error message returned by `GetUser()`,
  `GetUserByEmail()` and `GetUserByPhoneNumber()` APIs in
  [`auth`](https://godoc.org/firebase.google.com/go/auth) package.

# v2.6.0

- [changed] Improved error handling in FCM by mapping more server-side
  errors to client-side error codes.
- [added] Added the `db` package for interacting with the Firebase database.

# v2.5.0

- [changed] Import context from `golang.org/x/net` for 1.6 compatibility

### Cloud Messaging

- [added] Added the `messaging` package for sending Firebase notifications
  and managing topic subscriptions.

### Authentication

- [added] A new [`VerifyIDTokenAndCheckRevoked()`](https://godoc.org/firebase.google.com/go/auth#Client.VerifyIDToken)
  function has been added to check for revoked ID tokens.
- [added] A new [`RevokeRefreshTokens()`](https://godoc.org/firebase.google.com/go/auth#Client.RevokeRefreshTokens)
  function has been added to invalidate all refresh tokens issued to a user.
- [added] A new property `TokensValidAfterMillis` has been added to the
  ['UserRecord'](https://godoc.org/firebase.google.com/go/auth#UserRecord)
  type, which stores the time of the revocation truncated to 1 second accuracy.

# v2.4.0

### Initialization

- [added] The [`firebase.NewApp()`](https://godoc.org/firebase.google.com/go#NewApp)
  method can now be invoked without any arguments. This initializes an app
  using Google Application Default Credentials, and
  [`firebase.Config`](https://godoc.org/firebase.google.com/go#Config) loaded
  from the `FIREBASE_CONFIG` environment variable.

### Authentication

- [changed] The user management operations in the `auth` package now uses the
  [`identitytoolkit/v3`](https://google.golang.org/api/identitytoolkit/v3) library.
- [changed] The `ProviderID` field on the
  [`auth.UserRecord`](https://godoc.org/firebase.google.com/go/auth#UserRecord)
  type is now set to the constant value `firebase`.

# v2.3.0

- [added] A new [`InstanceID`](https://godoc.org/firebase.google.com/go#App.InstanceID)
  API that facilitates deleting instance IDs and associated user data from
  Firebase projects.

# v2.2.1

### Authentication

-  [changed] Adding the `X-Client-Version` to the headers in the API calls for
  tracking API usage.

# v2.2.0

### Authentication

- [added] A new user management API that supports querying and updating
  user accounts associated with a Firebase project. This adds `GetUser()`,
  `GetUserByEmail()`, `GetUserByPhoneNumber()`, `CreateUser()`, `UpdateUser()`,
  `DeleteUser()`, `Users()` and `SetCustomUserClaims()` functions to the
  [`auth.Client`](https://godoc.org/firebase.google.com/go/auth#Client) API.

# v2.1.0

- [added] A new [`Firestore` API](https://godoc.org/firebase.google.com/go#App.Firestore)
  that enables access to [Cloud Firestore](/docs/firestore) databases.

# v2.0.0

- [added] A new [Cloud Storage API](https://godoc.org/firebase.google.com/go/storage)
  that facilitates accessing Google Cloud Storage buckets using the
  [`cloud.google.com/go/storage`](https://cloud.google.com/go/storage)
  package.

### Authentication

- [changed] The [`Auth()`](https://godoc.org/firebase.google.com/go#App.Auth)
  API now accepts a `Context` argument. This breaking
  change enables passing different contexts to different services, instead
  of using a single context per [`App`](https://godoc.org/firebase.google.com/go#App).

# v1.0.2

### Authentication

- [changed] When deployed in the Google App Engine environment, the SDK can
  now leverage the utilities provided by the
  [App Engine SDK](https://cloud.google.com/appengine/docs/standard/go/reference)
  to sign JWT tokens. As a result, it is now possible to initialize the Admin
  SDK in App Engine without a service account JSON file, and still be able to
  call [`CustomToken()`](https://godoc.org/firebase.google.com/go/auth#Client.CustomToken)
  and [`CustomTokenWithClaims()`](https://godoc.org/firebase.google.com/go/auth#Client.CustomTokenWithClaims).

# v1.0.1

### Authentication

- [changed] Now uses the client options provided during
  [SDK initialization](https://godoc.org/firebase.google.com/go#NewApp) to
  create the [`http.Client`](https://godoc.org/net/http#Client) that is used
  to fetch public key certificates. This enables developers to use the ID token
  verification feature in environments like Google App Engine by providing a
  platform-specific `http.Client` using
  [`option.WithHTTPClient()`](https://godoc.org/google.golang.org/api/option#WithHTTPClient).

# v1.0.0

- [added] Initial release of the Admin Go SDK. See
  [Add the Firebase Admin SDK to your Server](/docs/admin/setup/) to get
  started.
- [added] You can configure the SDK to use service account credentials, user
  credentials (refresh tokens), or Google Cloud application default credentials
  to access your Firebase project.

### Authentication

- [added] The initial release includes the `CustomToken()`,
  `CustomTokenWithClaims()`, and `VerifyIDToken()` functions for minting custom
  authentication tokens and verifying Firebase ID tokens.
