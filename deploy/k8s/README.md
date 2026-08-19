# Kubernetes マニフェスト（P04 workspace）

api / collab / web です。共同編集（Yjs）とチャット WS は別 Service です。このフォルダだけを apply しないでください。起動は [pf-cloud-k8s](https://github.com/maeplego/pf-cloud-k8s) の collab overlay からです。

| ホスト | 用途 |
| --- | --- |
| `workspace.localhost` | Web（OIDC 必須） |
| `workspace-api.localhost` | REST とチャット WS |
| `workspace-collab.localhost` | Yjs |

Compose 単体は開発ヘッダ認証です。単体デモは `deploy/compose.yaml` です。
