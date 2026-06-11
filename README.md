# viseu-agent

Agente de nó do Viseu AI Lab. Liga o seu computador à rede comunitária de
inferência: comunica ao gateway que modelos o seu motor local serve e
mantém essa informação atualizada. Quando o agente para, o nó desaparece
do catálogo passado um minuto. Partilha quem quer, quando quer.

Um único binário, sem instalação nem serviços. Disponível para macOS,
Windows e Linux nas [releases](https://github.com/viseuai/agent/releases).

## Antes de começar

1. **Adesão**: inicie sessão em https://platform.viseuai.org com a sua
   conta GitHub. O acesso de membro é atribuído pela associação.
2. **Chave de nó**: na consola, separador "Nós de computação", crie uma
   chave de nó (começa por `vsk_`). É mostrada uma única vez.
3. **Chave de rede**: peça a chave de acesso à rede privada a
   geral@viseuai.org. O agente liga-se sozinho à rede na primeira
   execução; não é preciso instalar mais nada.
4. **Motor de inferência** a correr localmente. Qualquer servidor
   compatível com a API da OpenAI serve. Exemplos abaixo.

## Com Ollama

O Ollama pode ficar como está: o agente faz a ponte entre a rede privada
e o seu computador, e o Ollama nunca fica exposto.

```sh
viseu-agent \
  -key vsk_a_sua_chave_de_no \
  -mesh-key chave_de_rede_recebida \
  -engine-url http://localhost:11434
```

Todos os modelos que o Ollama tiver instalados (`ollama list`) ficam
disponíveis na plataforma. Para parar de partilhar, Ctrl+C. Nas execuções
seguintes a chave de rede já não é necessária.

## Com llama.cpp

```sh
viseu-agent -key vsk_... -mesh-key chave_de_rede \
  -engine-url http://localhost:8090 \
  -engine-cmd "llama-server -m modelo.gguf --port 8090"
```

## Com MLX (Mac, Apple Silicon)

```sh
viseu-agent -key vsk_... -mesh-key chave_de_rede \
  -engine-cmd "mlx_lm.server --model mlx-community/Qwen2.5-3B-Instruct-4bit --port 8090"
```

## Opções

| Opção | Variável | Por omissão | Descrição |
|---|---|---|---|
| `-key` | `NODE_KEY` | obrigatória | Chave de nó (`vsk_...`) |
| `-mesh-key` | `MESH_KEY` | primeira execução | Chave de acesso à rede privada |
| `-engine-url` | `ENGINE_URL` | `http://localhost:8090` | URL local do motor |
| `-engine-cmd` | `ENGINE_CMD` | (nenhum) | Comando do motor, iniciado e vigiado pelo agente |
| `-name` | `NODE_NAME` | nome da máquina | Nome do nó na consola |
| `-gateway` | `GATEWAY_URL` | `https://api.viseuai.org` | Gateway da plataforma |
| `-mesh-port` | `MESH_PORT` | `8443` | Porta servida na rede privada |
| `-interval` | `INTERVAL_SECONDS` | `15` | Intervalo dos sinais de vida |
| `-advertise-url` | `ADVERTISE_URL` | (avançado) | Usar um cliente Tailscale do sistema em vez do nó embebido |

## Segurança

O seu computador nunca aceita ligações da internet: só o gateway da
associação o alcança, através da rede privada, e apenas para pedidos de
inferência. O agente comunica para fora por HTTPS. O conteúdo dos pedidos
segue a política de privacidade publicada em viseuai.org/privacidade.
