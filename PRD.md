# PRD - SimulaArch: Simulador Visual de Carga para Microsserviços

## 1. Visão Geral do Produto

O **SimulaArch** é uma ferramenta de engenharia de confiabilidade (SRE) e arquitetura que permite desenhar topologias de microsserviços e simular o comportamento do sistema sob carga variável. O foco é identificar gargalos de hardware, saturação de filas, latência de rede e efeitos de escalonamento de forma visual e teórica, sem necessidade de provisionar infraestrutura real.

## 2. Objetivos Principais

1. **Visualização de Arquitetura**  
   - Permitir o desenho de arquiteturas complexas via Drag & Drop em um canvas interativo.  
   - Representar visualmente tipos distintos de componentes (Clientes, Gateways, Serviços, Filas, Clusters, etc.).

2. **Simulação de Performance**  
   - Calcular o impacto de CPU, Memória, I/O e latência de rede em fluxos de requisições síncronas e assíncronas.  
   - Estimar métricas de latência agregadas (p50, p95, **p99**, throughput efetivo) por rota.

3. **Análise de Falha e Saturação**  
   - Simular o que acontece quando um serviço, fila ou cluster atinge ou ultrapassa sua capacidade máxima (CPU, throughput, conexões).  
   - Identificar e destacar gargalos de forma visual (ex.: nós em vermelho acima de 100%, amarelo acima de 80%).

4. **Custo Zero de Infraestrutura**  
   - Realizar simulações matemáticas sem necessidade de provisionar servidores, clusters ou filas reais.  
   - Permitir salvar e compartilhar cenários de simulação somente via arquivos JSON.

## 3. Personas

1. **Arquiteto de Soluções**  
   - Objetivo: Validar se o desenho de uma nova solução suportará o tráfego projetado.  
   - Necessidades:  
     - Modelar topologias end-to-end (cliente, gateways, serviços, filas, bancos/armazenamento).  
     - Rodar cenários de “e se” (aumento de tráfego, falha de um serviço, redução de replicas).  
     - Gerar relatórios ou screenshots para discussão com time técnico e stakeholders.

2. **Desenvolvedor Backend**  
   - Objetivo: Entender como o tempo de processamento e o uso de recursos do seu serviço afetam o tempo de resposta final do usuário (principalmente **p95/p99**).  
   - Necessidades:  
     - Ajustar parâmetros de processamento (tempo médio, variação, uso de CPU/RAM).  
     - Ver impacto da inclusão de chamadas remotas adicionais (N+1, fan-out).  
     - Comparar cenários com e sem cache, filas, retries.

3. **SRE/DevOps**  
   - Objetivo: Simular cenários de transbordamento de fila (backpressure), limites de rate limit e estratégias de escalonamento.  
   - Necessidades:  
     - Validar políticas de **HPA** (Horizontal Pod Autoscaler) e thresholds.  
     - Simular falha parcial de nós (queda de uma réplica, redução de throughput de uma fila).  
     - Estimar risco de saturação em picos de tráfego (eventos, campanhas, datas sazonais).

## 4. Requisitos Funcionais

### 4.1. Interface de Modelagem (Canvas)

1. **Paleta de Componentes**  
   - Deve permitir arrastar ícones representando pelo menos:  
     - Client  
     - API Gateway / Load Balancer  
     - Service (microsserviço)  
     - Queue/Topic (Kafka, RabbitMQ ou genérico)  
     - K8s Cluster / Compute Pool  
   - Cada tipo de componente deve ter um ícone ou cor distinta.

2. **Sistema de Conexões**  
   - Permitir unir componentes através de arestas direcionais:  
     - Request/Response (síncrono).  
     - Pub/Sub (assíncrono).  
   - As conexões devem carregar informações mínimas:  
     - Tipo (sync/async).  
     - Percentual de tráfego roteado (para fan-out/fan-in).  
     - Latência de rede opcional (ms) por aresta (default global se não especificado).

3. **Painel de Propriedades**  
   - Ao selecionar um componente, exibir painel lateral com parâmetros editáveis específicos do tipo (ver seção 4.2).  
   - Deve permitir:  
     - Editar valores numéricos via input (inteiro/float) com validação básica.  
     - Editar labels/nome lógico do componente.  
     - Visualizar rapidamente um resumo da capacidade teórica do nó (ex.: RPS máximo, utilização atual após simulação).

4. **Gestão de Cenários (MVP simples)**  
   - Criar novo diagrama em branco.  
   - Limpar diagrama atual.  
   - Upload de arquivo JSON para carregar arquitetura.  
   - Download do estado atual em JSON.

### 4.2. Componentes e Parâmetros de Simulação

Para o MVP, os seguintes componentes e parâmetros são obrigatórios:

1. **Client**  
   - Parâmetros editáveis:  
     - `RPS` (req/s)  
     - `PayloadSizeKB` (KB)  
     - `Concurrency` (número de conexões simultâneas)  
   - Lógica:  
     - Gera a carga inicial do sistema, distribuída nas conexões de saída conforme pesos/percentuais.

2. **API Gateway**  
   - Parâmetros editáveis:  
     - `RateLimitRPS`  
     - `LatencyOverheadMs`  
     - (Opcional pós-MVP) Estratégia em caso de excesso: Drop, 429, fila interna.  
   - Lógica:  
     - Filtra tráfego excedente acima de `RateLimitRPS` (considerar como drop/erro 429 no MVP).  
     - Adiciona overhead fixo de latência ao caminho.

3. **Service (Microsserviço)**  
   - Parâmetros editáveis:  
     - `CPU_Cores`  
     - `RAM_GB` (para o MVP pode ser apenas informativo)  
     - `ProcessTimeMs` (tempo médio de processamento por requisição)  
     - `MaxRPS` opcional (derivado de CPU_Cores e ProcessTimeMs por fórmula simples).  
   - Lógica:  
     - Calcular saturação com base em:  
       $$Utilizacao = rac{RPS_{entrada} 	imes ProcessTimeMs}{1000 	imes CPU\_Cores}$$  
     - Destacar visualmente quando `Utilizacao > 0.8` (80%) e quando `> 1.0` (100%).

4. **Queue (Kafka/Rabbit/Genérico)**  
   - Parâmetros editáveis:  
     - `ThroughputMaxMsgsPerSec`  
     - `WriteLatencyMs`  
     - (Opcional pós-MVP) Tamanho máximo da fila.  
   - Lógica:  
     - Se o produtor envia mais mensagens do que o throughput máximo e/ou consumidores retiram abaixo da taxa de produção, simular **lag** crescente.  
     - Apresentar métrica: `LagEst` (número de mensagens/segundo acumuladas).

5. **K8s Cluster / Compute Pool**  
   - Parâmetros editáveis:  
     - `MinReplicas`  
     - `MaxReplicas`  
     - `HPA_Threshold` (% de utilização alvo, ex.: 70%).  
     - (Opcional) `BaseCPUperReplica` (se não inferido do Service).  
   - Lógica:  
     - Ajustar capacidade total baseada na carga recebida.  
     - No MVP, aplicar regra simplificada:  
       - Calcular réplicas necessárias para manter utilização abaixo do threshold; truncar ao inteiro dentro de [MinReplicas, MaxReplicas].  
       - Recalcular capacidade efetiva e indicar se ainda há saturação.

### 4.3. Motor de Simulação (Backend em Go)

1. **Algoritmo de Fluxo**  
   - Processar o grafo de conexões para distribuir carga (RPS e tamanho de payload) de forma determinística.  
   - Assumir, no MVP, tráfego estacionário (sem componente temporal).  
   - Suportar:  
     - Caminhos síncronos (soma de latências em série).  
     - Componentes assíncronos via fila (producer -> queue -> consumer).

2. **Cálculo de Latência**  
   - Fórmula base:  
     $$LatenciaTotal = \sum (LatenciaProcessamento + LatenciaRede + TempoFila)$$  
   - No MVP, cada nó contribui com:  
     - `LatenciaProcessamento`: derivada do `ProcessTimeMs` ou latência de escrita/leitura em fila.  
     - `LatenciaRede`: valor padrão global configurável ou por aresta.  
     - `TempoFila`: função da utilização e do lag estimado (pode ser modelo simplificado M/M/1 ou aproximação linear).  
   - Objetivo: exibir pelo menos `LatenciaMedia` por rota e opcionalmente `p95`/`p99` por aproximação simples (ex.: fator multiplicador).

3. **Detecção de Gargalo**  
   - Critérios:  
     - Nó em **alerta** quando `Utilizacao >= 0.8`.  
     - Nó em **crítico** quando `Utilizacao >= 1.0`.  
   - Saída:  
     - Backend retorna, por nó, campos como `utilization`, `status` (OK/ALERT/CRITICAL) e métricas derivadas (RPS efetivo, lag, latência).  
     - Frontend colore nós com base em `status` (ex.: verde, amarelo, vermelho).

4. **Execução da Simulação**  
   - A simulação é disparada pelo usuário (botão "Rodar Simulação").  
   - O backend recebe o JSON completo da arquitetura e retorna um JSON com métricas calculadas por nó e por aresta.  
   - O frontend atualiza a visualização (cores, tooltips com números, dashboards simples).

## 5. Requisitos Não Funcionais

1. **Stack Tecnológica (MVP)**  
   - Backend: Go (Golang) utilizando `html/template` para Server-Side Rendering.  
   - Frontend: HTML + CSS + JavaScript Vanilla, podendo usar **D3.js** ou SVG para o canvas de modelagem.

2. **Persistência**  
   - Sem banco de dados no MVP.  
   - O estado da arquitetura deve ser **exportável/importável via JSON**:  
     - Download de um arquivo `.json` contendo a estrutura da arquitetura.  
     - Upload de um arquivo `.json` para restaurar o estado anterior.

3. **Desempenho**  
   - O motor de simulação deve processar grafos de até **50 nós** em **menos de 100ms** no backend (desconsiderando latência de rede).  
   - O carregamento inicial da página deve ocorrer em < 2s em ambiente de rede comum (indicativo, não bloqueante para MVP).

4. **Usabilidade**  
   - Interface deve ser utilizável em tela única (desktop) com resolução mínima de 1366x768.  
   - Operações principais (arrastar componente, conectar nós, editar propriedades, rodar simulação) devem estar acessíveis em até 2 cliques.

5. **Confiabilidade da Simulação**  
   - O modelo matemático pode ser simplificado, mas deve ser **determinístico** para os mesmos inputs.  
   - Assumir distribuição de chegadas e tempos de serviço estacionária (sem variação temporal) no MVP.

## 6. Arquitetura de Dados Proposta

O sistema deve tratar a arquitetura como um **Grafo Direcionado**.

```go
type Architecture struct {
    Nodes []Node `json:"nodes"`
    Edges []Edge `json:"edges"`
}

type Node struct {
    ID     string                 `json:"id"`
    Type   string                 `json:"type"`   // "client", "gateway", "service", "queue", "cluster", etc.
    Config map[string]interface{} `json:"config"` // Ex.: {"cpu_cores": 2, "rps": 500}
}

type Edge struct {
    ID           string                 `json:"id"`
    From         string                 `json:"from"`
    To           string                 `json:"to"`
    Type         string                 `json:"type"`          // "sync" ou "async"
    TrafficShare float64                `json:"trafficShare"` // 0.0 a 1.0
    Config       map[string]interface{} `json:"config"`       // Ex.: {"latency_ms": 5}
}
```

- A simulação opera sobre uma instância de `Architecture`, propagando carga a partir de nós do tipo `client`.

## 7. Roadmap de Desenvolvimento (MVP)

### Sprint 1 (Fundação da UI e Backend)

- Estrutura básica do servidor Go com `html/template`.  
- Canvas básico com Drag & Drop (JS) e paleta mínima de componentes (Client, Service, Gateway).  
- Criação/remoção/movimentação de nós no canvas.  
- Criação/remoção de conexões direcionais entre nós.  
- Estrutura inicial do modelo de dados em JSON (Architecture, Node, Edge).

### Sprint 2 (Motor de Cálculo Síncrono)

- Implementação do motor de cálculo de carga simples (somente Request/Response síncrono).  
- Cálculo de utilização de CPU em Services baseado em RPS de entrada.  
- Indicação visual de saturação (cores nos nós) para fluxos simples, ex.: `Client -> Gateway -> Service`.  
- Endpoint HTTP para receber JSON da arquitetura e devolver métricas calculadas.

### Sprint 3 (Componentes Assíncronos e Persistência)

- Adição de componentes assíncronos (Filas/Mensageria) ao canvas.  
- Modelagem de producer/consumer via Queue.  
- Cálculo de lag estimado em filas.  
- Implementação de exportação JSON (download) e importação (upload) da arquitetura.  
- Ajustes no motor para suportar mistura de caminhos síncronos e assíncronos.

### Sprint 4 (Refino Visual e Métricas Avançadas)

- Refinamento visual de alertas de overload (legendas, tooltips, ícones).  
- Exibição de relatórios de latência por rota (latência média e p95/p99 por aproximação).  
- Painel de resumo com principais gargalos (top N nós mais saturados).  
- Pequenas melhorias de UX (atalhos, zoom no canvas, pan).

## 8. Critérios de Aceite

1. **Modelagem Básica de Fluxo Síncrono**  
   - Deve ser possível desenhar, salvar e simular pelo menos a rota:  
     `Client -> Gateway -> Service`  
   - Ao rodar a simulação, o sistema calcula utilização do Service e latência total do caminho.

2. **Saturação de Service por Aumento de RPS**  
   - Dado um Service com `CPU_Cores` e `ProcessTimeMs` configurados, ao aumentar o `RPS` do Client acima da capacidade teórica, o Service deve indicar visualmente quando:  
     - `Utilizacao >= 0.8` (alerta).  
     - `Utilizacao >= 1.0` (crítico/"CPU estourou").

3. **Simulação com Filas (após Sprint 3)**  
   - Deve ser possível modelar um fluxo producer -> queue -> consumer.  
   - Ao configurar um throughput máximo na fila inferior ao tráfego produzido, o sistema deve exibir um lag crescente na Queue.

4. **Persistência via JSON**  
   - O usuário deve conseguir:  
     - Baixar um arquivo JSON que representa totalmente a arquitetura modelada.  
     - Carregar esse mesmo arquivo JSON e restaurar o diagrama e configurações exatamente como estavam.  
   - Esse fluxo deve funcionar sem necessidade de qualquer banco de dados no backend.

5. **Desempenho do Motor de Simulação**  
   - Para um grafo com até 50 nós e 100 arestas, o tempo de processamento da simulação no backend deve ser **<= 100ms** em ambiente de referência.  
   - Em caso de falha na simulação (erro de grafo, ciclo inválido, configuração inconsistente), o sistema deve retornar mensagem de erro clara em vez de travar a interface.

---

Este PRD refina e detalha o documento original, mantendo a proposta de valor do **SimulaArch** e tornando os requisitos mais explícitos e verificáveis para implementação do MVP.
