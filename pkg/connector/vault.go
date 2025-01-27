package connector

import (
	"context"

	onepassword "github.com/ConductorOne/baton-1password/pkg/1password"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	ent "github.com/conductorone/baton-sdk/pkg/types/entitlement"
	grant "github.com/conductorone/baton-sdk/pkg/types/grant"
	resource "github.com/conductorone/baton-sdk/pkg/types/resource"
)

// 1Password Teams and 1Password Families.
var basicPermissions = map[string]string{
	"allow_viewing":  "allow viewing",
	"allow_editing":  "allow editing",
	"allow_managing": "allow managing",
}

// 1Password Business.
var businessPermissions = map[string]string{
	"view_items":              "view items",
	"create_items":            "create items",
	"edit_items":              "edit items",
	"archive_items":           "archive items",
	"delete_items":            "delete items",
	"view_and_copy_passwords": "view and copy passwords",
	"view_item_history":       "view item history",
	"import_items":            "import items",
	"export_items":            "export items",
	"copy_and_share_items":    "copy and share items",
	"print_items":             "print items",
	"manage_vault":            "manage vault",
}

const businessAccountType = "BUSINESS"

type vaultResourceType struct {
	resourceType *v2.ResourceType
	cli          *onepassword.Cli
}

func (g *vaultResourceType) ResourceType(_ context.Context) *v2.ResourceType {
	return g.resourceType
}

// Create a new connector resource for a 1Password vault.
func vaultResource(vault onepassword.Vault, parentResourceID *v2.ResourceId) (*v2.Resource, error) {
	ret, err := resource.NewResource(
		vault.Name,
		resourceTypeVault,
		vault.ID,
		resource.WithParentResourceID(parentResourceID),
	)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func (g *vaultResourceType) List(_ context.Context, parentId *v2.ResourceId, _ *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
	if parentId == nil {
		return nil, "", nil, nil
	}

	var rv []*v2.Resource

	vaults, err := g.cli.ListVaults()
	if err != nil {
		return nil, "", nil, err
	}

	for _, vault := range vaults {
		vaultCopy := vault
		gr, err := vaultResource(vaultCopy, parentId)
		if err != nil {
			return nil, "", nil, err
		}
		rv = append(rv, gr)
	}

	return rv, "", nil, nil
}

func (g *vaultResourceType) Entitlements(_ context.Context, resource *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	var rv []*v2.Entitlement

	account, err := g.cli.GetAccount()
	if err != nil {
		return nil, "", nil, err
	}

	memberOptions := PopulateOptions(resource.DisplayName, memberEntitlement, resource.Id.ResourceType)
	memberEntitlement := ent.NewAssignmentEntitlement(resource, memberEntitlement, memberOptions...)
	rv = append(rv, memberEntitlement)

	// Business accounts have more granular permissions.
	if account.Type == businessAccountType {
		for _, permission := range businessPermissions {
			businessOptions := PopulateOptions(resource.DisplayName, permission, resource.Id.ResourceType)
			businessEntitlement := ent.NewPermissionEntitlement(resource, permission, businessOptions...)
			rv = append(rv, businessEntitlement)
		}
	} else {
		for _, permission := range basicPermissions {
			basicOptions := PopulateOptions(resource.DisplayName, permission, resource.Id.ResourceType)
			basicEntitlement := ent.NewPermissionEntitlement(resource, permission, basicOptions...)
			rv = append(rv, basicEntitlement)
		}
	}

	return rv, "", nil, nil
}

func (g *vaultResourceType) Grants(_ context.Context, resource *v2.Resource, _ *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	var rv []*v2.Grant
	var userPermissionGrant *v2.Grant
	var groupPermissionGrant *v2.Grant

	account, err := g.cli.GetAccount()
	if err != nil {
		return nil, "", nil, err
	}

	vaultMembers, err := g.cli.ListVaultMembers(resource.Id.Resource)
	if err != nil {
		return nil, "", nil, err
	}

	for _, member := range vaultMembers {
		memberCopy := member
		ur, err := userResource(memberCopy, resource.Id)
		if err != nil {
			return nil, "", nil, err
		}

		membershipGrant := grant.NewGrant(resource, memberEntitlement, ur.Id)
		rv = append(rv, membershipGrant)

		for _, permission := range member.Permissions {
			if account.Type == businessAccountType {
				userPermissionGrant = grant.NewGrant(resource, businessPermissions[permission], ur.Id)
			} else {
				userPermissionGrant = grant.NewGrant(resource, basicPermissions[permission], ur.Id)
			}
			rv = append(rv, userPermissionGrant)
		}
	}

	vaultGroups, err := g.cli.ListVaultGroups(resource.Id.Resource)
	if err != nil {
		return nil, "", nil, err
	}

	for _, group := range vaultGroups {
		groupCopy := group
		groupMembers, err := g.cli.ListGroupMembers(groupCopy.ID)
		if err != nil {
			return nil, "", nil, err
		}

		for _, member := range groupMembers {
			memberCopy := member
			ur, err := userResource(memberCopy, resource.Id)
			if err != nil {
				return nil, "", nil, err
			}

			membershipGrant := grant.NewGrant(resource, memberEntitlement, ur.Id)
			rv = append(rv, membershipGrant)

			// add group permissions to all users in the group.
			for _, permission := range group.Permissions {
				if account.Type == businessAccountType {
					groupPermissionGrant = grant.NewGrant(resource, businessPermissions[permission], ur.Id)
				} else {
					groupPermissionGrant = grant.NewGrant(resource, basicPermissions[permission], ur.Id)
				}
				rv = append(rv, groupPermissionGrant)
			}
		}
	}

	return rv, "", nil, nil
}

func vaultBuilder(cli *onepassword.Cli) *vaultResourceType {
	return &vaultResourceType{
		resourceType: resourceTypeVault,
		cli:          cli,
	}
}
