# **Multi-Tenancy and Tenant-Scoped Support in Auth API**

## **Overview**

The Firebase Auth API supports multi-tenancy (Google Cloud Identity Platform), allowing a single Firebase project to host distinct "tenants" with their own users and configuration. The Go SDK mirrors this by providing two distinct client interfaces that share the same underlying implementation:

1. **auth.Client**: Performs operations at the **Project level**.  
2. **auth.TenantClient**: Performs operations at the **Tenant level**.

## **Implementation Pattern: baseClient**

To avoid code duplication, most functionality (user management, token verification, etc.) is implemented in a private struct called **baseClient**.

* **auth.Client** embeds \*baseClient directly.  
* **auth.TenantClient** also embeds \*baseClient.

### **The tenantID Field**

The baseClient struct contains a tenantID string field.

* For **auth.Client**, this field is empty "".  
* For **auth.TenantClient**, this field is populated with the specific Tenant ID.

### **Dynamic URL Construction**

Agents must implement API calls within baseClient using helper methods (like `makeUserMgtURL`) that dynamically construct URLs based on the presence of `tenantID`.

**Logic:**

* **If tenantID is empty:** Construct a project-level URL.  
  * Example: `/projects/{projectID}/accounts`  
* **If tenantID is set:** Construct a tenant-level URL.  
  * Example: `/projects/{projectID}/tenants/{tenantID}/accounts`

**Example Implementation (auth/user\_mgt.go):**

```go

func (c \*baseClient) makeUserMgtURL(path string) (string, error) {  
	if c.projectID \== "" {  
		return "", errors.New("project id not available")  
	}

	var url string  
	if c.tenantID != "" {  
		url = fmt.Sprintf("%s/projects/%s/tenants/%s%s", c.userManagementEndpoint, c.projectID, c.tenantID, path)  
	} else {  
		url = fmt.Sprintf("%s/projects/%s%s", c.userManagementEndpoint, c.projectID, path)  
	}

	return url, nil  
}
```

### **Token Verification**

When verifying ID tokens (`VerifyIDToken`), the baseClient checks the token's firebase.tenant claim:

* If the client has a tenantID set, it **must** match the token's tenant claim.  
* If the token has a tenant claim but the client does not (or vice versa), verification fails with tenantIDMismatch.

## **Testing Requirements for Agents**

Because the logic is shared but the endpoints differ, **separate tests are strictly required** to ensure the tenantID is correctly propagated to the API calls.

### **1\. Project-Level Tests**

* **Location:** `auth/auth_test.go` or `auth/user_mgt_test.go`.  
* **Purpose:** Verify specific functionality for the default Client.  
* **Verification:** Ensure the request URL **does not** contain `/tenants/`.

### **2\. Tenant-Scoped Tests**

* **Location:** `auth/tenant_mgt_test.go`.  
* **Purpose:** Verify that the same method, when called on a `TenantClient`, correctly targets the tenant endpoint.  
* **Verification:** Ensure the request URL **contains** `/tenants/{tenantID}/`.

**Example Test Pattern:**

If you add a new method `DeleteUser(ctx, uid)` to baseClient:

1. **Project Test:** Add a test in `user_mgt_test.go` calling `client.DeleteUser(...)`. Assert the mock server received a request at `/projects/p/accounts:delete`.  
2. **Tenant Test:** Add a test in `tenant_mgt_test.go` calling `tenantClient.DeleteUser(...)`. Assert the mock server received a request at `/projects/p/tenants/t/accounts:delete`.

## **Summary Checklist for Agents**

* \[ \] Implement new functionality in `baseClient`.  
* \[ \] Use or update URL construction helpers (like `makeUserMgtURL`) to handle `c.tenantID`.  
* \[ \] Add standard tests in `auth_test.go` or `user_mgt_test.go` checking project-level endpoints.  
* \[ \] Add tenant-specific tests in `tenant_mgt_test.go` checking tenant-level endpoints.
