# skills-browser

TUIブラウザ for SKILL.md files — シングルバイナリ、依存ゼロで動作。

## ビルド

### Go がある場合

```bash
# 依存を取得
go mod download

# ローカル向けバイナリを生成
make build          # → ./skills-browser

# クロスコンパイル
make build-linux    # → ./skills-browser_linux_amd64
make build-mac      # → ./skills-browser_darwin_arm64 / amd64
make build-windows  # → ./skills-browser_windows_amd64.exe
```

### Docker でビルド（Go 不要）

```bash
make docker-build   # → ./skills-browser_linux_amd64
```

## 使い方

```bash
# デフォルト: カレントディレクトリを検索
./skills-browser

# ディレクトリを指定
./skills-browser /path/to/your/repo/skills
```

## キーバインド

| キー | 動作 |
|------|------|
| `↑` / `k` | 前のスキルへ |
| `↓` / `j` | 次のスキルへ |
| `PgUp` / `Ctrl+U` | 詳細ペインを上にスクロール |
| `PgDn` / `Ctrl+D` | 詳細ペインを下にスクロール |
| `q` / `Ctrl+C` | 終了 |

## SKILL.md の `Arguments` セクションについて

以下のいずれかの形式で書くと右ペインに自動表示されます。

### マークダウンテーブル形式

```markdown
## Arguments

| Name   | Type   | Required | Description       |
|--------|--------|----------|-------------------|
| input  | string | required | 入力ファイルのパス |
| output | string | optional | 出力先ディレクトリ |
```

### 箇条書き形式

```markdown
## Arguments

- `input` (string, required): 入力ファイルのパス
- `output` (string, optional): 出力先ディレクトリ
```
