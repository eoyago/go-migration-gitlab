# Documentação do Script de Migração de Projetos e Grupos do GitLab

## Visão Geral
Este script foi projetado para:
- Migrar projetos e grupos do GitLab de um servidor fonte para um servidor alvo.
- Arquivar projetos e grupos.
- Replicar variáveis de ambiente, issues, releases e proteger branches.

---

## Funcionalidades

### Migração de Projetos
- Migra todos os projetos de um grupo e seus subgrupos do servidor fonte para o alvo.
- Cria o espelhamento (mirror) dos projetos entre servidores.
- Migra:
  - Variáveis de ambiente.
  - Branches protegidos.
  - Issues.
  - Releases.

### Migração de Grupos
- Identifica e recria grupos e subgrupos no servidor alvo.
- Replica variáveis de ambiente entre grupos.

### Arquivamento de Projetos
- Arquiva projetos especificados após a migração.

### Listagem de Projetos
- Lista projetos de um grupo para diagnóstico.

---

## Estrutura e Lógica do Script

### Variáveis Globais
- **Tokens e URLs**: Autenticação e base dos servidores.
- **Configurações de Migração**: Permite habilitar/desabilitar:
  - Migração de issues.
  - Migração de releases.
- **Níveis de Acesso**: Define permissões de branches protegidos.

### Função Principal
- **`main()`**: Controla a execução do script, habilitando funcionalidades como:
  - Listagem.
  - Migração.
  - Arquivamento.

---

## Funções Principais

### Migração de Variáveis de Ambiente
```go
func migrateAllVariablesByGroup()
```
 - Conecta-se aos servidores fonte e alvo.
 - Identifica variáveis de ambiente no grupo fonte.
 - Cria variáveis ausentes no grupo alvo.
### Migração de Projetos
```go
func migrateAllProjectsByGroupAndSubGroups()
```
 - Carrega projetos do grupo raiz e subgrupos.
 - Replica variáveis, branches protegidos e espelhamento.
## Sub-funções:
```go
createRecursiveSubGroupsAndProjects():
```
Cria subgrupos e migra projetos hierarquicamente.
```go
migrationAllProjectsByGroup():
```
Processa variáveis, branches e espelhamento.

### Casos de Uso

#### Migração Completa:

1. Use migrateAllProjectsByGroupAndSubGroups() para migrar todos os projetos, grupos e subgrupos.
Apenas Variáveis de Ambiente:

2. Use migrateAllVariablesByGroup() para migrar apenas variáveis de ambiente.
Arquivamento de Projetos:

3. Use archiveProjectsByGroup() para arquivar projetos após a migração.
Diagnóstico:

4. Use listProjectsByGroupAll() para listar projetos e releases.

### Exemplo de Execução

#### Configuração
Atualize as variáveis globais para incluir os tokens e URLs corretos.
Habilite a funcionalidade desejada no main().
#### Execução

Compile e execute o script:

```bash
go run main.go
```

## Notas
⚠️ Atenção:
O script exige tokens de autenticação válidos com permissões administrativas nos servidores fonte e alvo.
Certifique-se de testar o script em um ambiente de homologação antes de migrar dados de produção.