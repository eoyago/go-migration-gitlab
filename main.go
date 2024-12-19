package main

import (
	"log"
	"net/http"
	"strings"

	"github.com/xanzy/go-gitlab"
)

var (
	source_token            = "<SOURCE TOKEN>"
	source_base_url         = "<SOURCE_GITLAB_URL>"
	source_group_path       = "<SOURCE_GITLAB_GROUP>"
	source_branch_migration = "feat-dev-99999-gitlab-migration"

	target_token      = "<TARGET TOKEN>"
	target_base_url   = "<TARGET_GITLAB_URL>"
	target_group_path = "<TARGET_GITLAB_GROUP>"
	target_user       = "<TARGET_USERNAME>"
	// colocar igual a true para migrar releases
	target_create_release = false
	target_create_issues  = false

	maintainer_access   = gitlab.AccessLevelValue(40)
	no_one_access       = gitlab.AccessLevelValue(0)
	branch_wildcard_all = "*"

	sourceClient *gitlab.Client
	targetClient *gitlab.Client
)

func main() {

	// Apenas Listar os Projetos de um grupo:
	//listProjectsByGroupAll()

	// Migra todos projetos de um grupo e todos projetos dos subgrupos
	migrateAllProjectsByGroupAndSubGroups()

	// Apenas para listar projetos de um grupo e arquiva-los, use com cuidado!!!
	//archiveProjectsByGroup(createClientGitlab(source_base_url, source_token), source_group_path)

	// Migra as variaveis de ambiente de um grupo para outro grupo
	//migrateAllVariablesByGroup()
}

func migrateAllVariablesByGroup() {

	// cria uma conexão com o gitlab_source
	gitlabClient := createClientGitlab(target_base_url, target_token)

	// pega o id do projeto do gitlab_source
	sourceGroupId := getGroupIdByPath(gitlabClient, source_group_path)

	// pega o id do grupo do gitlab_target
	targetGroupId := getGroupIdByPath(gitlabClient, target_group_path)

	// carrega as variables do grupo do gitlab_target
	variablesByGroupSource := getVariablesByGroup(gitlabClient, sourceGroupId)
	// carrega as variaveis do grupo do gitlab_target
	variablesByGroupTarget := getVariablesByGroup(gitlabClient, targetGroupId)

	// faz um interação entre as variaveis do grupo gitlab_source para replicarmos as variables para o novo do gitlab_target
	for _, variableGroup := range variablesByGroupSource {

		// verifica se a variavel existe no gitlab_target, se existir da skip, se n existir cria
		if !containsVariableByGroup(variablesByGroupTarget, variableGroup.Key) {
			// cria variavel no gitlab_target
			createVariableByGroup(gitlabClient, targetGroupId, variableGroup)
		}
	}

}

func migrateAllProjectsByGroupAndSubGroups() {
	// cria uma conexão com o gitlab_source
	sourceClient = createClientGitlab(source_base_url, source_token)

	// cria uma conexão com o gitlab_target
	targetClient = createClientGitlab(target_base_url, target_token)

	// Lógica de migração dos projetos, mirror, variaveis ci/cd, tags, releases, issues e arquivar projeto migrado:
	// carrega os projetos do grupo definido acima, em gitlab_source
	sourceProjects := listProjectsByGroup(sourceClient, source_group_path)

	// pega o id do projeto do gitlab_source
	sourceGroupId := getGroupIdByPath(sourceClient, source_group_path)

	// pega o id do grupo do gitlab_target
	targetGroupId := getGroupIdByPath(targetClient, target_group_path)

	// mmigrando os projetos do path raiz
	migrationAllProjectsByGroup(sourceGroupId, targetGroupId, sourceProjects, target_group_path)

	// criando os subgrupos e projetos subsequentes
	createRecursiveSubGroupsAndProjects(sourceGroupId, targetGroupId, target_group_path)
}

func createRecursiveSubGroupsAndProjects(sourceGroupId int, targetGroupId int, groupPathFull string) {
	// lista todos os SubGrupos da path atual no gitlab_source
	sourceGroups := listSubGroupsByParent(sourceClient, sourceGroupId)

	for _, group := range sourceGroups {
		// cria o subgrupo caso nao exista
		targetGroup, exist := createGroupByTarget(group, targetGroupId)

		// monta o caminho do subgrupo atual com base nos subgrupos anterior
		group_path := groupPathFull + "/" + group.Path

		if exist {
			// carrega o subgrupo ja criado anteriormente
			targetGroup = getGroupByPath(targetClient, group_path)
		}

		// carrega os projetos do subgrupo atual do gitlab_source
		sourceProjects := listProjectsByGroup(sourceClient, group.FullPath)

		// migra os projetos listados no subgrupo atual
		migrationAllProjectsByGroup(group.ID, targetGroup.ID, sourceProjects, group_path)

		// carrega os proximos subgrupos a partir do subgrupo atual
		createRecursiveSubGroupsAndProjects(group.ID, targetGroup.ID, group_path)
	}
}

func migrationAllProjectsByGroup(sourceGroupId int, targetGroupId int, sourceProjects []*gitlab.Project, groupPath string) {

	// carrega as variables do grupo do gitlab_target
	variablesByGroupSource := getVariablesByGroup(sourceClient, sourceGroupId)
	// carrega as variaveis do grupo do gitlab_target
	variablesByGroupTarget := getVariablesByGroup(targetClient, targetGroupId)

	// faz um interação entre as variaveis do grupo gitlab_source para replicarmos as variables para o novo do gitlab_target
	for _, variableGroup := range variablesByGroupSource {

		// verifica se a variavel existe no gitlab_target, se existir da skip, se n existir cria
		if !containsVariableByGroup(variablesByGroupTarget, variableGroup.Key) {
			// cria variavel no gitlab_target
			createVariableByGroup(targetClient, targetGroupId, variableGroup)
		}
	}

	for _, project := range sourceProjects {

		// verifica se a branch default esta vazia, se estiver vazia e um projeto sem branch
		if project.DefaultBranch == "" {
			log.Printf("Projeto %s nao possui branch default", project.Name)
			continue
		}

		// realiza criação do projeto no novo gitlab_target
		projectTarget, exist := createProjectByTarget(targetClient, targetGroupId, project)

		if exist {
			// monta o path do projeto no gitlab_target
			projectPath := groupPath + "/" + project.Path
			// busca os dados do projeto no gitlab_target
			projectTarget = getProjectByPath(targetClient, projectPath)
		}

		//carrega as branchs protegidas do projeto gitlab_source
		protectedBranchesSource := getProtectedsBranchesByProject(sourceClient, project.ID)
		//carrega as branchs protegidas do projeto gitlab_target
		protectedBranchesTarget := getProtectedsBranchesByProject(targetClient, projectTarget.ID)

		// faz um interação proteções de branch do projeto no gitlab_source para replicarmos as regras o novo do gitlab_target
		for _, protectedBranch := range protectedBranchesSource {
			// se proteção de branch não existir ele cria no projeto do gitlab_target
			if !containsProtectedBranchProject(protectedBranchesTarget, protectedBranch.Name) {
				createProtectedBranchByProject(targetClient, projectTarget.ID, maintainer_access, protectedBranch.Name)
			}
		}

		// busca variaveis do projeto gitlab_source
		projectSourceVariables := getVariablesByProject(sourceClient, project.ID)
		// busca variaveis do projeto gitlab_target
		projectTargetVariables := getVariablesByProject(targetClient, projectTarget.ID)

		// interacao entre as variaveis do projeto gitlab_source para replicarmos as variaveis do novo do gitlab_target
		for _, variable := range projectSourceVariables {

			// verifica se a variavel existe no gitlab_target, se existir da skip, se n existir cria
			if !containsVariableByProject(projectTargetVariables, variable.Key) {
				// cria variavel no projeto do gitlab_target
				createVariableByProject(targetClient, projectTarget.ID, variable)
			}
		}

		// verifica se o projeto de origem ja tem algum mirror
		if !verifyExistMirrorProject(sourceClient, project.ID) {
			// cria o mirror no projeto de origem gitlab_source
			createProjectMirror(sourceClient, project.ID, project.Name, projectTarget)
			// inicia o mirror no projeto de origem
			startMirror(sourceClient, project.ID, project.Name, project.DefaultBranch)
		}

		// TODO: verificar se a regra de branch * no one push e merge ja esta criada no projeto de origem
		// TODO: se não estiver criar a regra de branch * no one push e merge
		if !containsProtectedBranchProject(protectedBranchesSource, branch_wildcard_all) {
			createProtectedBranchByProject(sourceClient, project.ID, no_one_access, branch_wildcard_all)
		}

		if target_create_release {
			migrationReleasesByProject(sourceClient, project, targetClient, projectTarget)
		}

		if target_create_issues {
			migrateIssuesByProject(sourceClient, targetClient, project, projectTarget)
		}

		archiveProject(sourceClient, project.ID)
	}
}

func migrationReleasesByProject(sourceClient *gitlab.Client, project *gitlab.Project, targetClient *gitlab.Client, projectTarget *gitlab.Project) {
	// busca as releases do projeto gitlab_source
	releasesSource := getReleasesByProject(sourceClient, project.ID)
	releasesTarget := getReleasesByProject(targetClient, projectTarget.ID)

	// se for diferente de nil e existir pelo menos uma release, realiza a interação
	if releasesSource != nil && len(releasesSource) > 0 {
		for _, release := range releasesSource {
			if !containsReleaseByProject(releasesTarget, release.Name) {
				createReleaseByProject(targetClient, projectTarget.ID, release)
			}
		}
	}
}

func containsReleaseByProject(releases []*gitlab.Release, releaseName string) bool {
	for _, release := range releases {
		if release.Name == releaseName {
			return true
		}
	}
	return false
}

func getVariablesByGroup(client *gitlab.Client, groupId int) []*gitlab.GroupVariable {
	variables, _, err := client.GroupVariables.ListVariables(groupId, nil)
	if err != nil {
		log.Fatalf("Erro ao buscar variáveis do grupo %d. Erro: %s\n", groupId, err)
	}

	log.Printf("Variables do grupo %d listadas com sucesso!\n", groupId)

	return variables
}

func containsProtectedBranchProject(protectedBranches []*gitlab.ProtectedBranch, protectedBranch string) bool {
	for _, branch := range protectedBranches {
		if branch.Name == protectedBranch {
			return true
		}
	}
	return false
}

func containsVariableByProject(variables []*gitlab.ProjectVariable, key string) bool {
	for _, variable := range variables {
		if variable.Key == key {
			return true
		}
	}
	return false
}

func containsVariableByGroup(variables []*gitlab.GroupVariable, key string) bool {
	for _, variable := range variables {
		if variable.Key == key {
			return true
		}
	}
	return false
}

func createVariableByProject(client *gitlab.Client, projectId int, variableSource *gitlab.ProjectVariable) {
	client.ProjectVariables.CreateVariable(projectId, &gitlab.CreateProjectVariableOptions{
		Key:   gitlab.Ptr(variableSource.Key),
		Value: gitlab.Ptr(variableSource.Value),
	})
}

func createVariableByGroup(client *gitlab.Client, groupId int, variableSource *gitlab.GroupVariable) {
	_, _, err := client.GroupVariables.CreateVariable(groupId, &gitlab.CreateGroupVariableOptions{
		Key:   gitlab.Ptr(variableSource.Key),
		Value: gitlab.Ptr(variableSource.Value),
	})

	if err != nil {
		log.Printf("Erro ao criar variável %s\n", variableSource.Key)
	}

	log.Printf("Variável %s criada com sucesso!\n", variableSource.Key)
}

func createClientGitlab(baseURL string, token string) *gitlab.Client {
	client, err := gitlab.NewClient(token, gitlab.WithBaseURL(baseURL))
	if err != nil {
		log.Fatal(err)
	}
	return client
}

func listProjectsByGroup(client *gitlab.Client, groupPath string) []*gitlab.Project {
	listGroupOptions := &gitlab.ListGroupProjectsOptions{
		IncludeSubGroups: gitlab.Ptr(false),
		Archived:         gitlab.Ptr(false),
		ListOptions: gitlab.ListOptions{
			PerPage: 100,
			//Sort:    "asc",
		},
	}

	projects, _, err := client.Groups.ListGroupProjects(groupPath, listGroupOptions)
	if err != nil {
		log.Fatal(err)
	}
	return projects
}

func listSubGroupsByParent(client *gitlab.Client, parentId int) []*gitlab.Group {
	subGroups, _, err := client.Groups.ListSubGroups(parentId, nil)
	if err != nil {
		log.Fatal(err)
	}
	return subGroups
}

func getGroupIdByPath(client *gitlab.Client, groupPath string) int {
	group, _, err := client.Groups.GetGroup(groupPath, &gitlab.GetGroupOptions{})
	if err != nil {
		log.Fatal(err)
	}
	return group.ID
}

func getProjectByPath(client *gitlab.Client, projectPath string) *gitlab.Project {
	project, _, err := client.Projects.GetProject(projectPath, nil)
	if err != nil {
		log.Fatal(err)
	}

	return project
}

func getGroupByPath(client *gitlab.Client, groupPath string) *gitlab.Group {
	group, _, err := client.Groups.GetGroup(groupPath, &gitlab.GetGroupOptions{})
	if err != nil {
		log.Fatal(err)
	}

	return group
}

func createGroupByTarget(group *gitlab.Group, targetGroupId int) (*gitlab.Group, bool) {
	groupTarget, _, err := targetClient.Groups.CreateGroup(&gitlab.CreateGroupOptions{
		Name:        gitlab.Ptr(group.Name),
		Path:        gitlab.Ptr(group.Path),
		ParentID:    gitlab.Ptr(targetGroupId),
		Visibility:  gitlab.Ptr(gitlab.PrivateVisibility),
		Description: gitlab.Ptr(group.Description),
	})

	if err != nil {
		errorGitlab, ok := err.(*gitlab.ErrorResponse)

		if !ok {
			log.Fatalln(err)
		}

		if errorGitlab.Response.StatusCode == http.StatusBadRequest && strings.Contains(errorGitlab.Message, "has already been taken") {
			log.Printf("Grupo %s ja existe!\n", group.Name)
			return nil, true
		}
	}

	log.Printf("GitLab group created successfully within group. ID: %d, Name: %s, URL: %s \n", groupTarget.ID, groupTarget.Name, groupTarget.WebURL)
	return groupTarget, false
}

func createProjectByTarget(client *gitlab.Client, targetGroupId int, project *gitlab.Project) (*gitlab.Project, bool) {
	projectTarget, _, err := client.Projects.CreateProject(&gitlab.CreateProjectOptions{
		Name:          gitlab.Ptr(project.Name),
		Path:          gitlab.Ptr(project.Path),
		Description:   gitlab.Ptr(project.Description),
		NamespaceID:   gitlab.Ptr(targetGroupId),
		DefaultBranch: gitlab.Ptr(project.DefaultBranch),
	})
	if err != nil {
		errorGitlab, ok := err.(*gitlab.ErrorResponse)

		if !ok {
			log.Fatalln(err)
		}

		if errorGitlab.Response.StatusCode == http.StatusBadRequest && strings.Contains(errorGitlab.Message, "has already been taken") {
			log.Printf("Projeto %s ja existe!\n", project.Name)
			return nil, true
		}

	}

	log.Printf("GitLab project created successfully within group. ID: %d, Name: %s, URL: %s \n", project.ID, projectTarget.Name, projectTarget.HTTPURLToRepo)

	return projectTarget, false
}

func getProtectedsBranchesByProject(client *gitlab.Client, projectId int) []*gitlab.ProtectedBranch {
	protectedBranches, _, err := client.ProtectedBranches.ListProtectedBranches(projectId, nil)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Branchs protegidas do projeto %d listadas com sucesso!\n", projectId)
	return protectedBranches
}

func createProtectedBranchByProject(client *gitlab.Client, projectId int, accessLevel gitlab.AccessLevelValue, branchName string) {
	protectedBranchOptions := &gitlab.ProtectRepositoryBranchesOptions{
		Name:             gitlab.Ptr(branchName),
		PushAccessLevel:  gitlab.Ptr(accessLevel),
		MergeAccessLevel: gitlab.Ptr(accessLevel),
	}

	_, _, err := client.ProtectedBranches.ProtectRepositoryBranches(projectId, protectedBranchOptions)
	if err != nil {
		log.Printf("Erro ao criar branchs protegida %s\n", branchName)
	}

	log.Printf("Branch %s protegida com sucesso", branchName)
}

func createProjectMirror(client *gitlab.Client, sourceProjectId int, sourceProjectName string, target *gitlab.Project) {
	urlPath := strings.TrimPrefix(target.HTTPURLToRepo, "https://")

	urlGitlabProjectMirror := "https://" + target_user + ":" + target_token + "@" + urlPath
	mirrorOptions := &gitlab.AddProjectMirrorOptions{
		URL:     gitlab.Ptr(urlGitlabProjectMirror),
		Enabled: gitlab.Ptr(true),
	}

	projectTarget, _, err := client.ProjectMirrors.AddProjectMirror(sourceProjectId, mirrorOptions)

	if err != nil {
		log.Fatal(err)
	}

	log.Printf("\nMirror no Project %s criado com sucesso. URL: %s \n", sourceProjectName, projectTarget.URL)
}

func startMirror(client *gitlab.Client, sourceProjectId int, sourceProjectName string, sourceBranchDefault string) {
	if verifyBranchExist(client, sourceProjectId) {
		deleteBranch(client, sourceProjectId)
	}

	log.Printf("Gerando alteração no projeto %s\n", sourceProjectName)
	_, _, err := client.Branches.CreateBranch(sourceProjectId, &gitlab.CreateBranchOptions{
		Branch: gitlab.Ptr(source_branch_migration),
		Ref:    gitlab.Ptr(sourceBranchDefault),
	})

	if err != nil {
		log.Fatalln(err)
	}

	//time.Sleep(2 * time.Second)
	//
	//deleteBranch(client, sourceProjectId)
}

func deleteBranch(client *gitlab.Client, sourceProjectId int) {
	_, err := client.Branches.DeleteBranch(sourceProjectId, source_branch_migration)
	if err != nil {
		log.Fatalln(err)
	}
}

func verifyBranchExist(client *gitlab.Client, sourceProjectId int) bool {
	_, _, err := client.Branches.GetBranch(sourceProjectId, source_branch_migration)
	if err != nil {
		return false
	}
	return true
}

func getVariablesByProject(client *gitlab.Client, projectId int) []*gitlab.ProjectVariable {
	variables, _, err := client.ProjectVariables.ListVariables(projectId, nil)
	if err != nil {
		log.Fatal(err)
	}
	return variables
}

func verifyExistMirrorProject(client *gitlab.Client, targetProjectId int) bool {
	mirrors, _, err := client.ProjectMirrors.ListProjectMirror(targetProjectId, nil)
	if err != nil {
		return false
	}
	if mirrors == nil || len(mirrors) == 0 {
		return false
	}

	log.Printf("Existe mirror do projeto %d, pulando criação\n", targetProjectId)

	return true
}

func getReleasesByProject(client *gitlab.Client, projectId int) []*gitlab.Release {
	listReleasesOptions := &gitlab.ListReleasesOptions{}

	releases, _, err := client.Releases.ListReleases(projectId, listReleasesOptions)
	if err != nil {
		log.Fatal(err)
	}
	return releases
}

func createReleaseByProject(client *gitlab.Client, projectId int, release *gitlab.Release) {
	_, _, err := client.Releases.CreateRelease(projectId, &gitlab.CreateReleaseOptions{
		Name:        gitlab.Ptr(release.Name),
		Description: gitlab.Ptr(release.Description),
		TagName:     gitlab.Ptr(release.TagName),
		ReleasedAt:  release.ReleasedAt,
	})
	if err != nil {
		log.Fatalf("Erro ao criar a Release no gitlab_target. Project: %d, Release: %s, Erro: %s\n", projectId, release.Name, err)
		return
	}

	log.Printf("Release criada no gitlab_target. Project: %d, Release: %s\n", projectId, release.Name)
}

func listProjectsByGroupAll() {
	sourceClient := createClientGitlab(source_base_url, source_token)
	sourceProjects := listProjectsByGroup(sourceClient, source_group_path)

	for _, project := range sourceProjects {
		releases := getReleasesByProject(sourceClient, project.ID)

		for _, release := range releases {
			log.Printf("Project: %s, Release: %s\n", project.Name, release.Name)
		}
	}
}

func migrateIssuesByProject(sourceClient *gitlab.Client, targetClient *gitlab.Client, project *gitlab.Project, projectTarget *gitlab.Project) {
	listIssues := &gitlab.ListProjectIssuesOptions{
		State: gitlab.Ptr("opened"),
		ListOptions: gitlab.ListOptions{
			PerPage: 100,
		},
	}

	// Obter issues do projeto sourcegit
	issuesSource, _, err := sourceClient.Issues.ListProjectIssues(project.ID, listIssues)
	if err != nil {
		log.Fatalf("Erro ao obter issues do projeto %s no sourcegit. Erro: %s\n", project.Name, err)
		return
	}

	// Se existirem issues, realizar a migração
	if len(issuesSource) > 0 {
		log.Printf("Iniciando migração de issues para o projeto %s\n", projectTarget.Name)

		for _, issueSource := range issuesSource {
			// Criar a issue no projeto target
			_, _, err := targetClient.Issues.CreateIssue(projectTarget.ID, &gitlab.CreateIssueOptions{
				Title:       gitlab.Ptr(issueSource.Title),
				Description: gitlab.Ptr(issueSource.Description),
				Labels:      gitlab.Ptr(issueSource.Labels),
			})

			if err != nil {
				log.Printf("Erro ao criar issue no projeto %s. Erro: %s\n", projectTarget.Name, err)
			} else {
				log.Printf("Issue migrada com sucesso para o projeto %s.\n", projectTarget.Name)
			}
		}

		log.Printf("Migração de issues concluída para o projeto %s\n", projectTarget.Name)
	} else {
		log.Printf("O projeto %s não possui issues para migrar\n", project.Name)
	}
}

func archiveProject(client *gitlab.Client, projectId int) {
	project, resp, err := client.Projects.ArchiveProject(projectId)
	if err != nil {
		log.Printf("Erro ao arquivar projeto. Project ID: %d, Erro: %s\n", projectId, err)
		return
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		log.Printf("Erro ao arquivar projeto. Project ID: %d, Código de resposta: %d\n", projectId, resp.StatusCode)
	} else {
		log.Printf("Projeto arquivado com sucesso. Project ID: %d\n", project.ID)
	}
}

func archiveProjectsByGroup(client *gitlab.Client, groupPath string) []*gitlab.Project {
	listGroupOptions := &gitlab.ListGroupProjectsOptions{
		IncludeSubGroups: gitlab.Ptr(true),
		Archived:         gitlab.Ptr(false),
		ListOptions: gitlab.ListOptions{
			PerPage: 100,
			//Sort:    "asc",
		},
	}

	projects, _, err := client.Groups.ListGroupProjects(groupPath, listGroupOptions)
	if err != nil {
		log.Fatal(err)
	}

	// Iterar sobre os projetos e arquivá-los
	for _, project := range projects {
		_, _, err := client.Projects.ArchiveProject(project.ID)
		if err != nil {
			log.Printf("Falha ao arquivar o projeto %s: %v", project.Name, err)
		} else {
			log.Printf("Projeto arquivado com sucesso: %s", project.Name)
		}
	}

	return projects
}
