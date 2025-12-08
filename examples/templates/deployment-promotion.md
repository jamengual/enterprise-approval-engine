## 🚀 {{.Title}}

{{- if .Description}}
{{.Description}}
{{- end}}

### Release Information

{{- if .Vars.service}}
- **Service:** {{.Vars.service}}
{{- end}}
{{- if .Version}}
- **Version:** `{{.Version}}`
{{- end}}
{{- if .RepoURL}}
- **Release URL:** [{{.Version}}]({{.RepoURL}}/releases/tag/{{.Version}})
{{- end}}
{{- if .Environment}}
- **{{.Environment | title}} Deployment:** ✅ Successful
{{- end}}
{{- if .Vars.deploy_url}}
- **{{.Environment | title}} URL:** {{.Vars.deploy_url}}
{{- end}}

---

### Deployment Commands

Use these commands in this issue's comments to deploy:

{{- if .Vars.environments}}
{{range $env := split .Vars.environments ","}}
- `.deploy {{$env}}` - Deploy to {{$env}} environment
{{- end}}
{{- else}}
- `.deploy qa` - Deploy to QA environment
- `.deploy staging` - Deploy to staging environment
- `.deploy production` - Deploy to production (requires approval)
{{- end}}
- `.noop <environment>` - Test deployment (dry run)

---

### Deployment Flow

✅ dev → ⏳ qa → ⏳ staging → ⏳ production

---

### Pre-deployment Checklist

- [ ] Dev testing completed
- [ ] Release notes reviewed
- [ ] No breaking changes (or migration plan ready)
- [ ] Stakeholders notified

---

### Approval Requirements

{{.GroupsTable}}

---

### How to Respond

**To Approve:** Comment `approve`, `approved`, `lgtm`, `yes`, or `/approve`

**To Deny:** Comment `deny`, `denied`, `reject`, `no`, or `/deny`

---

**Created by:** Release Workflow
{{- if .CommitSHA}}
**Commit:** [{{slice .CommitSHA 0 7}}]({{.CommitURL}})
{{- end}}
