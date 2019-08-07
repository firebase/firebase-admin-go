# Unreleased

# v3.9.0

- [added] Implemented `messaging.MulticastMessage` type and the
  `messaging.SendMulticast()` function for sending the same
  message to multiple recipients.
- [added] Implemented `messaging.SendAllDryRun()` and
  `messaging.SendMulticastDryRun()` functions for sending messages
  in the validate only mode.
- [added] Implemented `messaging.SendAll()` function for sending
  up to 100 FCM messages at a time.

# v3.8.1

- [fixed] Fixed a test case that was failing in environments without
  Application Default Credentials support.

# v3.8.0

- [added] Implemented `auth.EmailVerificationLink()` function for
  generating email verification action links.
- [added] Implemented `auth.PasswordResetLink()` function for
  generating password reset action links.
- [added] Implemented `auth.EmailSignInLink()` function for generating
  email sign in action links.

# v3.7.0

- [added] Implemented `auth.SessionCookie()` function for creating
  new Firebase session cookies given an ID token.
- [added] Implemented `auth.VerifySessionCookie()` and
  `auth.VerifySessionCookieAndCheckRevoked()` functions for verifying
  Firebase session cookies.
- [added] Implemented HTTP retries for the `db` package. This package
  now retries HTTP calls on low-level connection and socket read errors, as
  well as HTTP 500 and 503 errors.
- [fixed] Updated `messaging.Client` and `iid.Client` to use the new
  HTTP client API with retries support.

# v3.6.0

- [added] `messaging.Aps` type now supports critical sound in its payload.
- [fixed] Public types in the `messaging` package now support correct
  JSON marshalling and unmarshalling.
- [fixed] The `VerifyIDToken()` function fnow tolerates a clock skew of up to
  5 minutes when comparing JWT timestamps.

# v3.5.0

- [added] `messaging.AndroidNotification` type now supports `channel_id`.
- [dropped] Dropped support for Go 1.8 and earlier.
- [fixed] Fixing error handling in FCM. The SDK now checks the key
  `type.googleapis.com/google.firebase.fcm.v1.FcmError` to set error code.
- [added] `messaging.ApsAlert` type now supports subtitle in its payload.
- [added] `messaging.WebpushConfig` type now supports fcmOptions in its payload.

# v3.4.0

- [added] `firebase.App` now provides a new `DatabaseWithURL()` function
  for initializing a database client from a URL.

# v3.3.0

- [fixed] Fixing a regression introduced in 3.2.0, where `VerifyIDToken()`
  cannot be used in App Engine.
- [added] `messaging.WebpushNotification` type now supports arbitrary key-value
  pairs in its payload.

# v3.2.0

- [added] Implemented the ability to create custom tokens without
  service account credentials.
- [added] Added the `ServiceAccount` field to the `firebase.Config` struct.
- [added] The Admin SDK can now read the Firebase/GCP project ID from
  both `GCLOUD_PROJECT` and `GOOGLE_CLOUD_PROJECT` environment
  variables.
- [fixed] Using the default, unauthorized HTTP client to retrieve
  public keys when verifying ID tokens.

# v3.1.0

- [added] Added new functions for testing errors in the `iid` package
  (e.g. `iid.IsNotFound()`).
- [fixed] `auth.UpdateUser()` and `auth.DeleteUser()` return the expected
  `UserNotFound` error when called with a non-existing uid.
- [added] Implemented the `auth.ImportUsers()` function for importing
  users into Firebase Auth in bulk.

# v3.0.0

- [changed] All functions that make network calls now take context as an argument.

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
