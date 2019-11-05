# Contributing | Firebase Admin Go SDK

Thank you for contributing to the Firebase community!

 - [Have a usage question?](#question)
 - [Think you found a bug?](#issue)
 - [Have a feature request?](#feature)
 - [Want to submit a pull request?](#submit)
 - [Need to get set up locally?](#local-setup)


## <a name="question"></a>Have a usage question?

We get lots of those and we love helping you, but GitHub is not the best place for them. Issues
which just ask about usage will be closed. Here are some resources to get help:

- Go through the [guides](https://firebase.google.com/docs/admin/setup/)
- Read the full [API reference](https://godoc.org/firebase.google.com/go)

If the official documentation doesn't help, try asking a question on the
[Firebase Google Group](https://groups.google.com/forum/#!forum/firebase-talk/) or one of our
other [official support channels](https://firebase.google.com/support/).

**Please avoid double posting across multiple channels!**


## <a name="issue"></a>Think you found a bug?

Yeah, we're definitely not perfect!

Search through [old issues](https://github.com/firebase/firebase-admin-go/issues) before
submitting a new issue as your question may have already been answered.

If your issue appears to be a bug, and hasn't been reported,
[open a new issue](https://github.com/firebase/firebase-admin-go/issues/new). Please use the
provided bug report template and include a minimal repro.

If you are up to the challenge, [submit a pull request](#submit) with a fix!


## <a name="feature"></a>Have a feature request?

Great, we love hearing how we can improve our products! Share you idea through our
[feature request support channel](https://firebase.google.com/support/contact/bugs-features/).


## <a name="submit"></a>Want to submit a pull request?

Sweet, we'd love to accept your contribution!
[Open a new pull request](https://github.com/firebase/firebase-admin-go/pull/new/master) and fill
out the provided template.

Make sure to create all your pull requests against the `dev` branch. All development
work takes place on this branch, while the `master` branch is dedicated for released
stable code. This enables us to review and merge routine code changes, without
impacting downstream applications that are building against our `master`
branch.

**If you want to implement a new feature, please open an issue with a proposal first so that we can
figure out if the feature makes sense and how it will work.**

Make sure your changes pass our linter and the tests all pass on your local machine.
Most non-trivial changes should include some extra test coverage. If you aren't sure how to add
tests, feel free to submit regardless and ask us for some advice.

Finally, you will need to sign our
[Contributor License Agreement](https://cla.developers.google.com/about/google-individual),
and go through our code review process before we can accept your pull request.

### Contributor License Agreement

Contributions to this project must be accompanied by a Contributor License
Agreement. You (or your employer) retain the copyright to your contribution.
This simply gives us permission to use and redistribute your contributions as
part of the project. Head over to <https://cla.developers.google.com/> to see
your current agreements on file or to sign a new one.

You generally only need to submit a CLA once, so if you've already submitted one
(even if it was for a different project), you probably don't need to do it
again.

### Code reviews

All submissions, including submissions by project members, require review. We
use GitHub pull requests for this purpose. Consult
[GitHub Help](https://help.github.com/articles/about-pull-requests/) for more
information on using pull requests.

## <a name="local-setup"></a>Need to get set up locally?

### Initial Setup

Use the standard GitHub and [Go development tools](https://golang.org/doc/cmd)
to build and test the Firebase Admin SDK. Follow the instructions given in
the [golang documentation](https://golang.org/doc/code.html) to get your
`GOPATH` set up correctly. Then execute the following series of commands
to checkout the sources of Firebase Admin SDK, and its dependencies:

```bash
$ cd $GOPATH
$ git clone https://github.com/firebase/firebase-admin-go.git src/firebase.google.com/go
$ go get -d -t firebase.google.com/go/... # Install dependencies
```

### Unit Testing

Invoke the `go test` command as follows to build and run the unit tests:

```bash
go test -test.short firebase.google.com/go/...
```

Note the `-test.short` flag passed to the `go test` command. This will skip
the integration tests, and only execute the unit tests.

### Integration Testing

A suite of integration tests are available in the Admin SDK source code.
These tests are designed to run against an actual Firebase project. Create a new
project in the [Firebase Console](https://console.firebase.google.com), if you
do not already have one suitable for running the tests against. Then obtain the
following credentials from the project:

1. *Service account certificate*: This can be downloaded as a JSON file from
   the "Settings > Service Accounts" tab of the Firebase console. Click 
   "GENERATE NEW PRIVATE KEY" and copy the file into your Go workspace as
   `src/firebase.google.com/go/testdata/integration_cert.json`.
2. *Web API key*: This is displayed in the "Settings > General" tab of the
   console. Copy it and save to a new text file. Copy this text file into
   your Go workspace as
   `src/firebase.google.com/go/testdata/integration_apikey.txt`.

You'll also need to grant your service account the 'Firebase Authentication Admin' role. This is
required to ensure that exported user records contain the password hashes of the user accounts:
1. Go to [Google Cloud Platform Console / IAM & admin](https://console.cloud.google.com/iam-admin).
2. Find your service account in the list, and click the 'pencil' icon to edit it's permissions.
3. Click 'ADD ANOTHER ROLE' and choose 'Firebase Authentication Admin'.
4. Click 'SAVE'.

Now you can invoke the test suite as follows:

```bash
go test firebase.google.com/go/...
```

This will execute both unit and integration test suites.

### Test Coverage

Coverage can be measured per package by passing the `-cover` flag to the test invocation:

```bash
go test -cover firebase.google.com/go/auth
```

To view the detailed coverage reports (per package):

```bash
go test -cover -coverprofile=coverage.out firebase.google.com/go
go tool cover -html=coverage.out
```
