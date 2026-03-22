
## SPRINT 1 — Fundação da UI e Backend

### 1.1 Estrutura do Projeto
- [ ] Criar estrutura de diretórios do projeto Go:- [ ] Inicializar módulo Go (`go mod init`)
- [ ] Criar `main.go` com servidor HTTP básico na porta 8080
- [ ] Configurar roteamento de endpoints: `GET /` e `POST /simulate`
- [ ] Criar template HTML base (`templates/index.html`) com `html/template`
- [ ] Servir arquivos estáticos (`/static/`)

### 1.2 Modelos de Dados (Go)
- [ ] Criar `models/architecture.go` com as structs:
- `Architecture { Nodes []Node, Edges []Edge }`
- `Node { ID, Type, Label, X, Y float64, Config map[string]interface{} }`
- `Edge { ID, From, To, Type string, TrafficShare float64, Config map[string]interface{} }`
- [ ] Criar `models/simulation_result.go` com structs de resultado:
- `SimulationResult { Nodes []NodeResult, Edges []EdgeResult, Errors []string }`
- `NodeResult { ID, Status string, Utilization float64, EffectiveRPS float64, LatencyMs float64, Metrics map[string]interface{} }`
- `EdgeResult { ID string, RPSFlow float64, LatencyMs float64 }`
- [ ] Adicionar constantes de status: `OK`, `ALERT`, `CRITICAL`

### 1.3 Layout e Estrutura Visual da Interface
- [ ] Criar layout em três colunas no HTML/CSS:
- Coluna esquerda: Paleta de Componentes
- Centro: Canvas SVG/D3.js interativo
- Coluna direita: Painel de Propriedades
- [ ] Criar barra superior com: título do app, botões de ação (Novo, Limpar, Importar JSON, Exportar JSON, Rodar Simulação)
- [ ] Criar área de rodapé/status bar para mensagens e erros
- [ ] Aplicar CSS responsivo mínimo para resolução 1366x768+
- [ ] Definir paleta de cores e tipografia do projeto

### 1.4 Paleta de Componentes (Frontend)
- [ ] Criar sidebar esquerda com os 5 tipos de componentes arrastáveis:
- Client (ícone/cor: azul)
- API Gateway (ícone/cor: roxo)
- Service (ícone/cor: verde)
- Queue (ícone/cor: laranja)
- K8s Cluster (ícone/cor: ciano)
- [ ] Implementar ícones SVG inline distintos para cada tipo
- [ ] Adicionar tooltip descritivo ao passar o mouse sobre cada item da paleta
- [ ] Implementar evento `dragstart` em cada item da paleta

### 1.5 Canvas Interativo (Frontend - JS/SVG)
- [ ] Inicializar canvas SVG (ou D3.js) ocupando toda a área central
- [ ] Implementar `drop` no canvas para criar novo nó na posição soltada
- [ ] Renderizar cada nó como um grupo SVG `<g>` com:
- Retângulo/forma de fundo com cor do tipo
- Ícone SVG do tipo
- Label editável (nome do nó)
- [ ] Implementar movimentação de nós via drag dentro do canvas
- [ ] Implementar seleção de nó ao clicar (destacar borda)
- [ ] Implementar remoção de nó com tecla `Delete` ou botão no painel
- [ ] Gerar IDs únicos (`uuid` ou timestamp) para cada nó criado
- [ ] Manter estado interno dos nós em objeto JS (`state.nodes`)

### 1.6 Sistema de Conexões (Frontend)
- [ ] Implementar modo "conectar": ao clicar em porta de saída de um nó, iniciar aresta pendente
- [ ] Renderizar aresta pendente (linha tracejada) enquanto arrasta até o nó destino
- [ ] Finalizar aresta ao clicar no nó destino e adicionar ao `state.edges`
- [ ] Renderizar arestas como `<line>` ou `<path>` SVG com seta (`marker-end`) indicando direção
- [ ] Diferenciar visualmente arestas síncronas (linha sólida) e assíncronas (linha tracejada)
- [ ] Implementar remoção de aresta ao selecionar e pressionar `Delete`
- [ ] Manter estado interno das arestas em `state.edges`
- [ ] Impedir conexão de um nó a ele mesmo

---

## SPRINT 2 — Motor de Cálculo Síncrono

### 2.1 Painel de Propriedades (Frontend)
- [ ] Exibir painel direito dinamicamente ao selecionar um nó
- [ ] Implementar campos editáveis por tipo de nó:
- **Client**: `RPS`, `PayloadSizeKB`, `Concurrency`
- **API Gateway**: `RateLimitRPS`, `LatencyOverheadMs`
- **Service**: `CPU_Cores`, `RAM_GB`, `ProcessTimeMs`
- **Queue**: `ThroughputMaxMsgsPerSec`, `WriteLatencyMs`
- **K8s Cluster**: `MinReplicas`, `MaxReplicas`, `HPA_Threshold`
- [ ] Implementar campo de nome/label editável para todos os tipos
- [ ] Validação básica de inputs (não negativos, inteiros onde aplicável)
- [ ] Atualizar `state.nodes[id].config` em tempo real ao editar campos
- [ ] Exibir seção "Resultado da Simulação" no painel (vazia antes de rodar)

### 2.2 Painel de Propriedades — Conexões
- [ ] Ao selecionar uma aresta, exibir painel com:
- Tipo de conexão: `sync` / `async` (radio ou select)
- `TrafficShare` (0.0 a 1.0 ou percentual)
- `LatencyMs` opcional por aresta
- [ ] Atualizar `state.edges[id]` ao editar campos da aresta

### 2.3 Motor de Simulação — Caminhos Síncronos (Go)
- [ ] Criar `simulator/engine.go` com função principal `Simulate(arch Architecture) SimulationResult`
- [ ] Implementar travessia do grafo por BFS/DFS a partir de nós do tipo `client`
- [ ] Implementar propagação de RPS através das arestas respeitando `TrafficShare`
- [ ] Implementar lógica do nó **Client**: propaga `RPS * TrafficShare` para cada aresta de saída
- [ ] Implementar lógica do nó **API Gateway**:
- Tráfego recebido é limitado a `RateLimitRPS`
- Excedente é descartado (drop/429)
- Adiciona `LatencyOverheadMs` ao caminho
- [ ] Implementar lógica do nó **Service**:
- Calcular `Utilizacao = (RPS_entrada * ProcessTimeMs) / (1000 * CPU_Cores)`
- Calcular `MaxRPS = (1000 * CPU_Cores) / ProcessTimeMs`
- Definir status: `OK` se < 0.8, `ALERT` se >= 0.8, `CRITICAL` se >= 1.0
- Retornar `EffectiveRPS`, `Utilization`, `LatencyMs`
- [ ] Implementar cálculo de latência total por rota (soma em série de `ProcessTimeMs + LatencyOverheadMs + latência por aresta`)
- [ ] Implementar detecção de ciclos no grafo (retornar erro claro se detectado)

### 2.4 Endpoint de Simulação (Go)
- [ ] Criar `handlers/simulate.go` com handler `POST /simulate`
- [ ] Fazer parse do body JSON para struct `Architecture`
- [ ] Validar estrutura recebida (nós sem ID, arestas com From/To inexistentes, etc.)
- [ ] Chamar `simulator.Simulate(arch)` e retornar `SimulationResult` como JSON
- [ ] Tratar erros e retornar HTTP 400 com mensagem descritiva em caso de grafo inválido
- [ ] Garantir tempo de resposta <= 100ms para grafos com até 50 nós

### 2.5 Integração Frontend → Simulação
- [ ] Implementar botão "Rodar Simulação" que serializa `state` para JSON e faz `POST /simulate`
- [ ] Exibir indicador de loading durante a requisição
- [ ] Ao receber resultado, atualizar visualmente cada nó no canvas com sua cor de status:
- Verde: `OK`
- Amarelo: `ALERT`
- Vermelho: `CRITICAL`
- [ ] Exibir tooltip sobre cada nó com: `Utilização`, `RPS efetivo`, `Latência estimada`
- [ ] Exibir métricas do nó selecionado no painel de propriedades (seção "Resultado")
- [ ] Exibir mensagens de erro da simulação na status bar

---

## SPRINT 3 — Componentes Assíncronos e Persistência

### 3.1 Componente Queue no Canvas
- [ ] Adicionar Queue à paleta de componentes
- [ ] Renderizar Queue com forma/ícone distinto no canvas
- [ ] Implementar campos editáveis no painel: `ThroughputMaxMsgsPerSec`, `WriteLatencyMs`
- [ ] Suportar conexão de qualquer nó como Producer (entrada na Queue)
- [ ] Suportar conexão da Queue para qualquer nó como Consumer (saída da Queue)
- [ ] Marcar aresta entrando na Queue como `async`

### 3.2 Componente K8s Cluster no Canvas
- [ ] Adicionar K8s Cluster à paleta de componentes
- [ ] Renderizar Cluster com forma distinta (grupo/container visual)
- [ ] Implementar campos editáveis: `MinReplicas`, `MaxReplicas`, `HPA_Threshold`
- [ ] Permitir que Services sejam associados a um Cluster (via config ou agrupamento)

### 3.3 Motor de Simulação — Filas Assíncronas (Go)
- [ ] Implementar lógica do nó **Queue** no motor:
- `IngressRPS` = soma de todos os producers
- `EgressRPS` = min(IngressRPS, ThroughputMaxMsgsPerSec)
- `LagEst` = max(0, IngressRPS - ThroughputMaxMsgsPerSec) (msgs/s acumuladas)
- Utilização = IngressRPS / ThroughputMaxMsgsPerSec
- Status: `OK/ALERT/CRITICAL` pelos mesmos thresholds (0.8 e 1.0)
- Latência de escrita = `WriteLatencyMs`
- [ ] Implementar lógica do nó **K8s Cluster**:
- Calcular réplicas necessárias: `ceil(Utilizacao / HPA_Threshold)`
- Limitar ao intervalo `[MinReplicas, MaxReplicas]`
- Recalcular capacidade efetiva com réplicas resultantes
- Indicar saturação se mesmo no MaxReplicas ainda há sobrecarga
- [ ] Adaptar travessia do grafo para tratar nós `async` (queue) como "barreiras" de fluxo
- [ ] Suportar mistura de caminhos síncronos e assíncronos na mesma arquitetura

### 3.4 Exportação e Importação JSON
- [ ] Implementar função JS `exportJSON()`:
- Serializar `state` completo (nós com posições, configs, arestas) para JSON
- Disparar download do arquivo `simulaarch.json` via Blob + `<a>` tag
- [ ] Implementar função JS `importJSON()`:
- Abrir seletor de arquivo (`<input type="file">`)
- Fazer parse do JSON carregado
- Validar estrutura mínima (campos obrigatórios)
- Restaurar `state` e re-renderizar canvas completamente
- [ ] Garantir que posições X/Y dos nós sejam salvas e restauradas
- [ ] Garantir que resultado de simulação anterior não seja persistido no JSON (apenas a arquitetura)
- [ ] Criar handler Go `GET /` que entrega o template principal (sem lógica de persistência server-side)

---

## SPRINT 4 — Refino Visual e Métricas Avançadas

### 4.1 Métricas de Latência Avançadas
- [ ] Implementar cálculo de `p95` e `p99` por aproximação simples no motor Go:
- `p95 = LatenciaMedia * 1.5` (ou fator configurável)
- `p99 = LatenciaMedia * 2.0` (ou fator configurável)
- [ ] Retornar `LatenciaMedia`, `p95`, `p99` por rota no `SimulationResult`
- [ ] Identificar e retornar todas as rotas distintas do grafo (do Client até folhas)
- [ ] Exibir métricas de latência por rota no painel ou em modal dedicado

### 4.2 Painel de Gargalos (Dashboard Summary)
- [ ] Criar área/modal de "Resumo da Simulação" exibido após rodar a simulação
- [ ] Listar top N nós mais saturados (ordenados por utilização decrescente)
- [ ] Exibir para cada gargalo: nome, tipo, utilização (%), status, RPS efetivo
- [ ] Destacar visualmente gargalos críticos com ícone de alerta (⚠️ / 🔴)
- [ ] Exibir total de tráfego descartado (drops no Gateway, lag nas Queues)

### 4.3 Refinamento Visual dos Nós
- [ ] Adicionar animação/pulso em nós com status `CRITICAL`
- [ ] Exibir badge de utilização (%) diretamente sobre o nó no canvas
- [ ] Implementar legenda visual no canto do canvas (Verde=OK, Amarelo=ALERT, Vermelho=CRITICAL)
- [ ] Adicionar tooltip rico ao passar o mouse sobre nó (sem necessidade de selecioná-lo):
- Nome, Tipo, Utilização, RPS entrada, RPS saída, Latência, Lag (se Queue)
- [ ] Colorir arestas conforme fluxo de carga (espessura proporcional ao RPS ou cor de status)

### 4.4 Melhorias de UX no Canvas
- [ ] Implementar zoom no canvas (scroll do mouse)
- [ ] Implementar pan no canvas (arrastar fundo com botão direito ou espaço + arrastar)
- [ ] Implementar botão "Centralizar/Fit" para ajustar zoom ao conteúdo
- [ ] Implementar atalho de teclado `Ctrl+Z` para desfazer última ação (ao menos remoção de nó)
- [ ] Implementar atalho `Ctrl+S` para exportar JSON
- [ ] Implementar seleção múltipla de nós (Shift+click ou seleção por área)
- [ ] Implementar alinhamento automático básico (snap to grid opcional)

### 4.5 Gestão de Cenários
- [ ] Implementar botão "Novo Diagrama" com confirmação (evitar perda de dados)
- [ ] Implementar botão "Limpar Canvas" com confirmação
- [ ] Permitir renomear o cenário atual (título editável na barra superior)
- [ ] Salvar nome do cenário no JSON exportado

---

## Tarefas Transversais (QA, Ajustes e Finalizações)

### T1. Testes e Validação do Motor (Go)
- [ ] Escrever testes unitários para `simulator/engine.go`:
- Teste: `Client -> Service` simples, verificar utilização
- Teste: `Client -> Gateway -> Service` com rate limit ativo
- Teste: `Client -> Queue -> Service` com lag
- Teste: grafo com ciclo (deve retornar erro)
- Teste: múltiplos clients, fan-out de tráfego
- Teste: K8s Cluster com HPA ativando scale-up
- Teste: grafo com 50 nós processado em < 100ms
- [ ] Escrever testes de integração para o endpoint `POST /simulate`

### T2. Validações de Frontend
- [ ] Impedir rodar simulação com canvas vazio (mostrar mensagem)
- [ ] Impedir rodar simulação sem nenhum nó do tipo `client`
- [ ] Validar que `TrafficShare` das arestas de saída de um nó soma <= 1.0 (warning se não)
- [ ] Mostrar erro claro se JSON importado for inválido ou incompleto
- [ ] Impedir upload de arquivo que não seja `.json`

### T3. Qualidade e Performance
- [ ] Garantir que re-renderização do canvas após simulação não cause flickering
- [ ] Verificar comportamento com grafos grandes (30-50 nós) no canvas
- [ ] Testar importação/exportação JSON round-trip (export → import → re-export devem ser iguais)
- [ ] Testar no Chrome, Firefox e Safari (mínimo)
- [ ] Verificar resolução mínima 1366x768 sem quebra de layout

### T4. Documentação Mínima
- [ ] Criar `README.md` com:
- Descrição do projeto
- Pré-requisitos (Go 1.21+)
- Instruções de `go run` / `go build`
- Como usar: paleta, canvas, simulação, export/import
- [ ] Documentar parâmetros de cada componente no README ou help inline na UI
- [ ] Adicionar comentários GoDoc nas funções públicas do motor de simulação

---

## Ordem de Prioridade Resumida

| Prioridade | Tarefa                                              |
|------------|-----------------------------------------------------|
| P0         | Estrutura do projeto, modelos de dados, servidor Go |
| P0         | Canvas com drag & drop, criação e remoção de nós    |
| P0         | Sistema de conexões direcionais                     |
| P0         | Motor síncrono (Client → Gateway → Service)         |
| P0         | Endpoint POST /simulate + integração visual         |
| P1         | Painel de propriedades editável por tipo            |
| P1         | Componente Queue + motor assíncrono                 |
| P1         | Export/Import JSON                                  |
| P1         | Componente K8s Cluster + lógica HPA                 |
| P2         | Métricas p95/p99 e painel de gargalos               |
| P2         | Zoom, pan, tooltips ricos, legenda visual           |
| P2         | Testes unitários e de integração                    |
| P3         | Atalhos de teclado, undo, snap to grid              |
| P3         | README e documentação                               |