package netlify

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/netlify/open-api/v2/go/models"
	"github.com/netlify/open-api/v2/go/plumbing/operations"
)

func resourceSite() *schema.Resource {
	return &schema.Resource{
		Create: resourceSiteCreate,
		Read:   resourceSiteRead,
		Update: resourceSiteUpdate,
		Delete: resourceSiteDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},

			"custom_domain": {
				Type:     schema.TypeString,
				Optional: true,
			},

			"deploy_url": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"account_slug": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},

			"account_name": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"repo": {
				Type:     schema.TypeList,
				MaxItems: 1,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"command": {
							Type:     schema.TypeString,
							Optional: true,
						},

						"deploy_key_id": {
							Type:     schema.TypeString,
							Optional: true,
						},

						"dir": {
							Type:     schema.TypeString,
							Optional: true,
						},

						"provider": {
							Type:     schema.TypeString,
							Required: true,
						},

						"repo_path": {
							Type:     schema.TypeString,
							Required: true,
						},

						"repo_branch": {
							Type:     schema.TypeString,
							Required: true,
						},

						"installation_id": {
							Type:     schema.TypeInt,
							Computed: true,
						},
					},
				},
			},
		},
	}
}

func resourceSiteCreate(d *schema.ResourceData, metaRaw interface{}) error {
	meta := metaRaw.(*Meta)

	// If we are trying to create a site using a private repository (i.e. not
	// a public_repo) then we need to get an installation id for the provider.
	// The easiest (and only way I can see) of doing this is by querying a user's
	// pre-existing sites and getting it from there.

	// If we have an "account_slug" set we use a different API path that lets
	// us create a site in a specific team. Unfortunately we have to duplicate
	// a lot of stuff because the types are totally different even though
	// structurally they are identical.
	var site *models.Site
	if v, ok := d.GetOk("account_slug"); ok {
		params := operations.NewCreateSiteInTeamParams()
		params.AccountSlug = v.(string)
		params.Site = resourceSite_setupStruct(d)
		resp, err := meta.Netlify.Operations.CreateSiteInTeam(params, meta.AuthInfo)
		if err != nil {
			return err
		}

		site = resp.Payload
	} else {
		params := operations.NewCreateSiteParams()
		params.Site = resourceSite_setupStruct(d)
		resp, err := meta.Netlify.Operations.CreateSite(params, meta.AuthInfo)
		if err != nil {
			return err
		}

		site = resp.Payload
	}

	d.SetId(site.ID)
	return resourceSiteRead(d, metaRaw)
}

func resourceSiteRead(d *schema.ResourceData, metaRaw interface{}) error {
	meta := metaRaw.(*Meta)
	params := operations.NewGetSiteParams()
	params.SiteID = d.Id()
	resp, err := meta.Netlify.Operations.GetSite(params, meta.AuthInfo)
	if err != nil {
		// If it is a 404 it was removed remotely
		if v, ok := err.(*operations.GetSiteDefault); ok && v.Code() == 404 {
			d.SetId("")
			return nil
		}

		return err
	}

	site := resp.Payload
	d.Set("name", site.Name)
	d.Set("custom_domain", site.CustomDomain)
	d.Set("deploy_url", site.DeployURL)
	d.Set("account_slug", site.AccountSlug)
	d.Set("account_name", site.AccountName)
	d.Set("repo", nil)

	if site.BuildSettings != nil && site.BuildSettings.RepoPath != "" {
		d.Set("repo", []interface{}{
			map[string]interface{}{
				"command":         site.BuildSettings.Cmd,
				"deploy_key_id":   site.BuildSettings.DeployKeyID,
				"dir":             site.BuildSettings.Dir,
				"provider":        site.BuildSettings.Provider,
				"repo_path":       site.BuildSettings.RepoPath,
				"repo_branch":     site.BuildSettings.RepoBranch,
				"installation_id": site.BuildSettings.InstallationID,
			},
		})
	}

	return nil
}

func resourceSiteUpdate(d *schema.ResourceData, metaRaw interface{}) error {
	params := operations.NewUpdateSiteParams()
	params.Site = resourceSite_setupStruct(d)
	params.SiteID = d.Id()

	meta := metaRaw.(*Meta)
	_, err := meta.Netlify.Operations.UpdateSite(params, meta.AuthInfo)
	if err != nil {
		return err
	}

	return resourceSiteRead(d, metaRaw)
}

func resourceSiteDelete(d *schema.ResourceData, metaRaw interface{}) error {
	meta := metaRaw.(*Meta)
	params := operations.NewDeleteSiteParams()
	params.SiteID = d.Id()
	_, err := meta.Netlify.Operations.DeleteSite(params, meta.AuthInfo)
	return err
}

// Returns the SiteSetup structure that can be used for creation or updating.
func resourceSite_setupStruct(d *schema.ResourceData) *models.SiteSetup {
	result := &models.SiteSetup{
		Site: models.Site{
			Name:         d.Get("name").(string),
			CustomDomain: d.Get("custom_domain").(string),
		},
	}

	// If we have a repo config, then configure that
	if v, ok := d.GetOk("repo"); ok {
		vL := v.([]interface{})
		repo := vL[0].(map[string]interface{})

		result.Repo = &models.RepoInfo{
			Cmd:            repo["command"].(string),
			DeployKeyID:    repo["deploy_key_id"].(string),
			Dir:            repo["dir"].(string),
			Provider:       repo["provider"].(string),
			RepoPath:       repo["repo_path"].(string),
			RepoBranch:     repo["repo_branch"].(string),
			InstallationID: int64(repo["installation_id"].(int)),
		}
	}

	return result
}
