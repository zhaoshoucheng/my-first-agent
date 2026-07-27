# 视频生成请求规格（Request Spec）

用一个 **JSON 文件**描述一次完整的视频生成请求，交给：

```bash
go run ./examples/video -spec ./path/to/spec.json
```

加载器：`domain/video/spec.go` 的 `LoadSpec()`。按扩展名判断格式：`.json` 走 JSON
（也支持 `.yaml`/`.yml`，字段名相同，本文档只给 JSON）。

> 这份文档就是给 AI / 人填 spec 用的参数清单。可填模板见
> [`examples/video/spec.example.json`](../examples/video/spec.example.json)。

---

## 后端与模型路由

model 名前缀决定走哪个后端（无需手写 provider）：

| model 前缀 | 后端 | provider |
|---|---|---|
| `veo-*` | Veo（Google，Vertex AI，genai SDK） | `gcp-vertex-ai` |
| `sora-*` | Sora（Azure OpenAI，REST） | `azure-openai` |
| `seedance-*` / `dreamina-*` | ModelArk（REST） | `modelark` |

各后端对字段的支持不同，下面字段表用 **Veo** / **ModelArk** 两列标注；Sora 的差异集中见
[「Sora（sora-2）参数对应」](#sorasora-2参数对应)一节。

---

## 字段清单

### 顶层

| 字段 | 类型 | 必填 | 说明 | Veo | ModelArk |
|---|---|---|---|---|---|
| `model` | string | 否* | 模型名；留空则用 `config.yaml` 的 `video.default_model` | ✓ | ✓ |
| `prompt` | string | 是 | 文本提示词 | ✓ | ✓ |
| `negative_prompt` | string | 否 | 反向提示，说明**不要**出现的内容 | ✓ | ✗（忽略） |

\* `model` 与配置默认值二者至少有一个。

### 图片元素

图片元素用 `ImageRef` 描述：`path`（本地文件，读字节）或 `url`（`gs://` 或
`http(s)://`）**二选一**；`mime` 可选，缺省按文件扩展名推断（`.png`/`.webp`/其余按
`image/jpeg`）。

| 字段 | 类型 | 说明 | Veo | ModelArk |
|---|---|---|---|---|
| `image` | ImageRef | 首帧（图生视频的起始画面） | ✓ | ✓（编码为 base64 data URL） |
| `last_frame` | ImageRef | 尾帧 | ✓ | ✓（role=last_frame） |
| `reference_images` | RefImage[] | 参考图列表 | ✓（Veo 2，`role`=ASSET/STYLE） | ✓（一律当 reference_image，`role` 忽略） |

`RefImage` = `ImageRef` 再加一个 `role` 字段：`ASSET`（内容/主体参考）或 `STYLE`
（风格参考）。留空默认 `ASSET`。

> ModelArk 的图片必须能取到字节或 URL；Veo 走 Vertex，`gs://` URI 或本地字节均可。
>
> ⚠️ Veo 上 `reference_images` 与 `image` / `last_frame` **互斥**，且仅 `veo-2.*` 支持。
> 详见下文[「组合约束与常见报错」](#组合约束与常见报错重要)。

### 生成参数

| 字段 | 类型 | 说明 | 取值 / 备注 | Veo | ModelArk |
|---|---|---|---|---|---|
| `resolution` | string | 分辨率 | `720p` / `1080p`，或 `1280x720` / `1920x1080`（ModelArk 会归一到 `720p`/`1080p`/`480p`） | ✓ | ✓ |
| `aspect_ratio` | string | 画面比例 | `16:9`（横） / `9:16`（竖） | ✓ | ✓（映射为 ratio） |
| `duration_seconds` | int | 时长（秒） | Veo 常见步长 4/6/8；ModelArk 默认 5 | ✓ | ✓ |
| `generate_audio` | bool | 是否生成音频 | **Veo 3 支持；Veo 2 无音频** | ✓ | ✓ |
| `seed` | int | RNG 种子 | 同输入 + 同种子 → 结果稳定 | ✓ | ✓ |
| `fps` | int | 帧率 | | ✓ | ✗（忽略） |
| `number_of_videos` | int | 输出视频数 | 0 = 后端默认（当前只落盘第一个） | ✓ | ✗（忽略） |
| `person_generation` | string | 人物生成策略 | `dont_allow` / `allow_adult` | ✓ | ✗（忽略） |
| `enhance_prompt` | bool | 是否启用提示词改写 | | ✓ | ✗（忽略） |
| `output_gcs_uri` | string | 结果直接写入的 GCS 桶 | 仅 Veo；本项目默认不设，走字节回传落盘 | ✓ | ✗（忽略） |

> 标 ✗（忽略）的字段在该后端上不生效，填了也不会报错，只是被丢弃。

### 不支持（C 档，刻意不做）

视频生视频、参考音频——spec 里没有对应字段，无法表达。

---

## 组合约束与常见报错（重要）

Veo 后端由 Vertex AI 校验：**单个参数合法、但组合不被该模型支持**时，会报
`Error 400 ... Status: FAILED_PRECONDITION, Message: The request is not supported by
this model`。已知约束：

1. **`image`（首帧）与 `reference_images` 互斥**。genai 官方约定：给了
   `reference_images` 就必须有 `prompt`，且 `image` / `last_frame` 都不支持。两者同时
   给 → FAILED_PRECONDITION。
2. **`reference_images` 是 Veo 2 的能力**，只在 `veo-2.*` 上支持（最多 3 张 `ASSET`，
   或 1 张 `STYLE`）。在 `veo-3.*` 上带参考图会被拒。要用参考图控角色，就把 `model`
   换成 `veo-2.*`。
3. **Veo 2 无音频**：换到 Veo 2 后 `generate_audio` 不生效。
4. **首帧比例要与 `aspect_ratio` 一致**：首帧图是 16:9 却把 `aspect_ratio` 设成 9:16
   （或反过来）可能出错。确定横屏/竖屏后，首帧图与 `aspect_ratio` 要匹配。

**按目的二选一：**

| 目的 | model | 图片元素 |
|---|---|---|
| 分镜首帧驱动（图生视频） | `veo-3.*` | 给 `image`；`reference_images` 留 `[]` |
| 参考图控角色 / 风格 | `veo-2.*` | 给 `reference_images`；`image` / `last_frame` 设 `null` |

---

## Sora（sora-2）参数对应

**同一个 spec 文件可直接驱动 Sora，只需把 `model` 改成 `sora-2` / `sora-2-pro`。**
Sora 的参数模型与 Veo 不同，本项目在 sora 后端里做了自动换算，其余 Veo 专属字段
「填了也不报错、只是忽略」。对应关系：

| spec 字段 | Sora 行为 |
|---|---|
| `model` | `sora-2` 或 `sora-2-pro` |
| `prompt` | 直接使用 |
| `resolution` + `aspect_ratio` | **仅文生视频时用**：合成一个 `size`（WxH）：竖屏(`9:16`)→`720x1280`/`1024x1792`；横屏→`1280x720`/`1792x1024`（含 `1080` 视为高清档）。`resolution` 若已写成 `1280x720` 这种 WxH 则直接透传 |
| `duration_seconds` | 归一到 Sora 允许档位 **4 / 8 / 12** 的最近值（默认 4）|
| `image` | 作为 `input_reference`（首帧，单张）上传。**Sora 要求参考图像素尺寸精确等于 `size`**，故会自动 cover-crop + 缩放到最接近的允许尺寸：**竖/横由图片自身宽高比决定**（避免拉伸失真），`resolution` 只决定清晰度档位，此时 spec 的 `aspect_ratio` 被忽略 |
| `generate_audio` | **忽略**：Sora 原生自带音频，无开关 |
| `seed` / `fps` / `number_of_videos` / `negative_prompt` / `person_generation` / `enhance_prompt` / `output_gcs_uri` | **忽略** |
| `last_frame` / `reference_images`（多角色） | **忽略**：Sora 不支持 |

Sora 允许的 `size`（**按模型不同**）：
- `sora-2`：`720x1280` / `1280x720`（仅 720p 两档）
- `sora-2-pro`：额外支持 `1024x1792` / `1792x1024`（1080 档）

`seconds`：`4` / `8` / `12`（两模型相同）。给 `sora-2` 传高清尺寸会被自动降到 720p 对应方向的尺寸
（避免 "Resolution … is not supported for model sora-2" 报错）；要 1080 请用 `sora-2-pro`。

首帧图（`image`）作为 `input_reference` 二进制部件上传（Azure v1 视频端点与 OpenAI schema 兼容），
上传前会自动 cover-crop + 缩放到与 `size` 精确一致的允许尺寸（仅用标准库处理 png/jpeg/gif）。

账号：走 **`azure-openai`** provider（Azure 上部署的 Sora）。凭据需要 `api_key`（Bearer）、
`base_url`（Azure 资源终端点，**必填**）、`api_version`：

```json
{
  "name": "sora-azure",
  "provider": "azure-openai",
  "credential": {
    "api_key": "...",
    "base_url": "https://<resource>.openai.azure.com",
    "api_version": "preview"
  }
}
```

> 端点：`POST {base}/openai/v1/videos?api-version=…` 提交、`GET .../videos/{id}` 轮询、
> `GET .../videos/{id}/content` 取件。
>
> ⚠️ `azure-openai` provider 同时被 gpt 文本模型用。若你既有 gpt 账号又有 sora 账号，
> 当前账号选择是「同 provider 按账号名排序取第一个」，可能选错——需要时给账号命名或
> 让我加「按账号名指定」的能力。

### Sora 示例

```json
{
  "model": "sora-2",
  "prompt": "A red fox running through a snowy forest at dawn, cinematic",
  "resolution": "1080p",
  "aspect_ratio": "16:9",
  "duration_seconds": 8
}
```

上例会被换算成 `size=1792x1024`、`seconds=8` 提交给 Sora。

---

## 填写要点（给 AI 的提示）

1. **最小可用**：只填 `model` + `prompt` 即可跑文生视频。
2. **图生视频**：加 `image`（首帧）；要控制结尾再加 `last_frame`。
3. **音频**：要带声音就 `generate_audio: true`，且 model 用 `veo-3.*`。
4. **首帧与参考图二选一**：`image` 和 `reference_images` 不能同用（见「组合约束」）；
   要参考图就换 `veo-2.*` 且不给首帧。
5. **不需要的字段设为 `null` / `0` / `""` 或整键删掉**——可选数值/布尔用指针实现，
   "不写/null"与"零值"有区别：`"generate_audio": false` 会显式关音频，不写则交给
   后端默认；`"image": null` 表示无首帧（文生视频）。
6. **图片路径**是相对进程工作目录（一般为仓库根）的路径，或绝对路径。

## 最小示例（文生视频）

```json
{
  "model": "veo-3.0-generate-001",
  "prompt": "A red fox running through a snowy forest, cinematic",
  "resolution": "1080p",
  "aspect_ratio": "16:9",
  "duration_seconds": 8,
  "generate_audio": true
}
```

## 图生视频示例（Veo 3，用分镜首帧）

> `image` 与 `reference_images` 不能同用，这里只给首帧（可选再加 `last_frame` 控结尾）。

```json
{
  "model": "veo-3.0-generate-001",
  "prompt": "The subject dances in the rain, neon city background",
  "image": { "path": "./assets/first-frame.jpg" },
  "last_frame": null,
  "reference_images": [],
  "resolution": "1080p",
  "aspect_ratio": "9:16",
  "duration_seconds": 6,
  "generate_audio": true,
  "seed": 123
}
```

## 参考图控角色示例（Veo 2，不能同时给首帧）

> 参考图是 Veo 2 能力，且不能与 `image` / `last_frame` 同用；Veo 2 无音频。

```json
{
  "model": "veo-2.0-generate-001",
  "prompt": "The subject dances in the rain, neon city background",
  "image": null,
  "last_frame": null,
  "reference_images": [
    { "path": "./assets/subject.jpg", "role": "ASSET" },
    { "path": "./assets/style-ref.jpg", "role": "STYLE" }
  ],
  "aspect_ratio": "9:16",
  "duration_seconds": 6,
  "seed": 123
}
```
