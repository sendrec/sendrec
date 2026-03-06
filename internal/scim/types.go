package scim

// SCIMUser represents a SCIM 2.0 User resource.
type SCIMUser struct {
	Schemas    []string    `json:"schemas"`
	ID         string      `json:"id"`
	ExternalID string      `json:"externalId,omitempty"`
	UserName   string      `json:"userName"`
	Name       SCIMName    `json:"name"`
	Emails     []SCIMEmail `json:"emails"`
	Active     bool        `json:"active"`
	Meta       *SCIMMeta   `json:"meta,omitempty"`
}

type SCIMName struct {
	Formatted  string `json:"formatted,omitempty"`
	GivenName  string `json:"givenName,omitempty"`
	FamilyName string `json:"familyName,omitempty"`
}

type SCIMEmail struct {
	Value   string `json:"value"`
	Primary bool   `json:"primary"`
}

type SCIMMeta struct {
	ResourceType string `json:"resourceType"`
	Location     string `json:"location"`
}

type SCIMListResponse struct {
	Schemas      []string   `json:"schemas"`
	TotalResults int        `json:"totalResults"`
	ItemsPerPage int        `json:"itemsPerPage"`
	StartIndex   int        `json:"startIndex"`
	Resources    []SCIMUser `json:"Resources"`
}

type SCIMError struct {
	Schemas []string `json:"schemas"`
	Status  string   `json:"status"`
	Detail  string   `json:"detail,omitempty"`
}

type SCIMPatchOp struct {
	Schemas    []string        `json:"schemas"`
	Operations []SCIMOperation `json:"Operations"`
}

type SCIMOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path,omitempty"`
	Value interface{} `json:"value,omitempty"`
}

const (
	UserSchema         = "urn:ietf:params:scim:schemas:core:2.0:User"
	ListResponseSchema = "urn:ietf:params:scim:api:messages:2.0:ListResponse"
	PatchOpSchema      = "urn:ietf:params:scim:api:messages:2.0:PatchOp"
	ErrorSchema        = "urn:ietf:params:scim:api:messages:2.0:Error"
	SPConfigSchema     = "urn:ietf:params:scim:schemas:core:2.0:ServiceProviderConfig"
	SchemaSchema       = "urn:ietf:params:scim:schemas:core:2.0:Schema"
)
