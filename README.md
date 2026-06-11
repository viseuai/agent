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
3. **Rede privada**: peça a chave de acesso à rede a geral@viseuai.org e
   ligue-se com o cliente Tailscale apontado ao servidor da associação:

   ```sh
   tailscale up --login-server https://mesh.viseuai.org --authkey CHAVE_RECEBIDA
   ```

   Anote o seu endereço na rede (`tailscale ip -4`, algo como 100.64.0.x).
4. **Motor de inferência** a correr localmente. Qualquer servidor
   compatível com a API da OpenAI serve. Exemplos abaixo.

## Com Ollama

O Ollama escuta, por omissão, apenas no próprio computador. Para a rede
o alcançar, defina `OLLAMA_HOST=0.0.0.0` antes de o iniciar (no Windows:
variável de ambiente do sistema; no macOS/Linux: `export OLLAMA_HOST=0.0.0.0`).
Depois:

```sh
viseu-agent \
  -key vsk_a_sua_chave_de_no \
  -engine-url http://localhost:11434 \
  -advertise-url http://SEU_IP_DE_REDE:11434
```

Todos os modelos que o Ollama tiver instalados (`ollama list`) ficam
disponíveis na plataforma. Para parar de partilhar, Ctrl+C.

## Com llama.cpp

```sh
llama-server -m modelo.gguf --host 0.0.0.0 --port 8090
viseu-agent -key vsk_... -engine-url http://localhost:8090 -advertise-url http://SEU_IP_DE_REDE:8090
```

## Com MLX (Mac, Apple Silicon)

O agente pode iniciar e vigiar o motor por si:

```sh
viseu-agent -key vsk_... -advertise-url http://SEU_IP_DE_REDE:8090 \
  -engine-cmd "mlx_lm.server --model mlx-community/Qwen2.5-3B-Instruct-4bit --host 0.0.0.0 --port 8090"
```

## Opções

| Opção | Variável | Por omissão | Descrição |
|---|---|---|---|
| `-key` | `NODE_KEY` | obrigatória | Chave de nó (`vsk_...`) |
| `-advertise-url` | `ADVERTISE_URL` | obrigatória | URL do motor na rede privada |
| `-engine-url` | `ENGINE_URL` | `http://localhost:8090` | URL local do motor |
| `-engine-cmd` | `ENGINE_CMD` | (nenhum) | Comando do motor, iniciado e vigiado pelo agente |
| `-name` | `NODE_NAME` | nome da máquina | Nome do nó na consola |
| `-gateway` | `GATEWAY_URL` | `https://api.viseuai.org` | Gateway da plataforma |
| `-interval` | `INTERVAL_SECONDS` | `15` | Intervalo dos sinais de vida |

## Segurança

O seu computador nunca aceita ligações da internet: só o gateway da
associação o alcança, através da rede privada, e apenas para pedidos de
inferência. O agente comunica para fora por HTTPS. O conteúdo dos pedidos
segue a política de privacidade publicada em viseuai.org/privacidade.
