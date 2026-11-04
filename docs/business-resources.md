# 外部测评与业务资源库

质量监控的“外部测评”同时保存测评入口和中转业务所需的资源。入口按用途归入三组、十二类，不自动注册、接入或调用第三方服务。

- 质量与选型：中转站检测、中转站排名、模型能力基准、服务状态、价格与规格。
- 接入与运营：上游与模型 API、支付与收款、邮件与身份认证、部署与存储、监控与备份。
- 采购与产品：订阅与数字采购、图片与产品灵感。

## 本批提供的链接

2026-11-04 查看公开页面。以下“资料已核对”仅表示查阅了公开功能说明，不代表完成注册、真实付费测试、供应能力验证或商户资质审查。访问失败是本次环境的观察，不能推断站点已永久停止服务。

| 入口 | 分类 | 功能与核对结果 |
| --- | --- | --- |
| [AI YCCC](https://ai.yccc.me/) | 图片与产品灵感 | 两次访问超时；保留“图片站”原备注，图片生成、图库或托管用途待核实。 |
| [OpenAI Status](https://status.openai.com/) | 服务状态 | 沿用已有入口，不重复添加。覆盖 OpenAI 的组件和事故记录，不覆盖其他厂商的所有模型。原备注单独保存。 |
| [ZPAY](https://zpayz.cn/) | 支付与收款 | 公开页面介绍支付宝、微信收款接入及易支付兼容接口。需分别比较开户费、平台费、通道费、准入及结算。 |
| [DuluPay](https://dulupay.com) | 支付与收款 | 首页、www 域名和 doc.html 返回 404，产品及结算能力待核实，保留原链接。 |
| [Zoho ZeptoMail](https://zeptomail.zoho.com.cn) | 邮件与身份认证 | 控制台访问超时，功能参考 [官方产品页](https://www.zoho.com.cn/zeptomail/)。它负责事务邮件发送，第三方登录需另行配置身份提供方。 |
| [Resend](https://resend.com/signup) | 邮件与身份认证 | 保留注册入口，功能参考 [产品主页](https://resend.com/)。邮件发送平台，使用情况标为“试用中”。 |
| [Cloudflare 控制台](https://dash.cloudflare.com/) | 部署与存储 | 控制台有访问限制，功能参考 [R2 产品页](https://www.cloudflare.com/products/r2/)。R2 可存放备份，控制台本身不是自动备份流程。 |
| [OVHcloud US](https://us.ovhcloud.com/) | 部署与存储 | VPS、独立服务器、公共云、网络和存储，可用于部署方案比较。 |
| [AI 比价](https://aibijia.org/) | 订阅与数字采购 | 聚合 AI 订阅、充值及数字商品报价；不同商品权益不能直接比较，更不能等同于 API token 成本。原地区选择备注原样保留，不作为已核实建议。 |
| [engy](https://engy.ai/pricing) | 上游与模型 API | 开放模型推理 API 与价格目录，分别展示输入、输出和缓存等费用；折扣需按其特定比较口径理解。 |
| [DMC Studio](https://dmc.cc/studio) | 图片与产品灵感 | Studio 仅返回 New API 应用壳，登录后体验待核实；[主页](https://dmc.cc/)说明其国模 API 服务。保留用户希望参考的 Studio 路径。 |
| [ZovoCard 商城](https://zovocard.com/mall) | 订阅与数字采购 | 返回访问限制，当前库存、数字交付与售后待核实，保留采购候选。 |

## 同类与互补资源

这些入口补充可比较的产品和开源方案，按公开功能选择，不按返佣或未经验证的质量排名推荐。

| 场景 | 扩展入口 | 有助于补足什么 |
| --- | --- | --- |
| 多模型和国模上游 | [OpenRouter](https://openrouter.ai/)、[SiliconFlow](https://www.siliconflow.com/)、[Alibaba Cloud Model Studio](https://www.alibabacloud.com/en/product/modelstudio) | 聚合路由、多模态推理、厂商平台直连的不同供给方式。地域、协议、数据政策与费用需分别比较。 |
| 图片、视频和音频供给 | [fal](https://fal.ai/)、[Replicate](https://replicate.com/) | 发现文本之外的产品能力，理解异步任务、文件保留及按次、时长或计算量收费的差异。 |
| 国内支付直连 | [微信支付商户平台](https://pay.weixin.qq.com/)、[支付宝开放平台](https://open.alipay.com/) | 对照聚合支付的中间成本与接入要求。 |
| 国际收款与订阅 | [Stripe Payments](https://stripe.com/payments)、[Paddle](https://www.paddle.com/) | 支付处理和 Merchant of Record 的区别；实际商品是否受理、结算与争议成本需先确认。 |
| 第三方登录与账号身份 | [Auth0](https://auth0.com/) | 区分发邮件与身份认证，了解社交登录、企业组织和多因素认证。 |
| 部署备选 | [Hetzner Cloud](https://www.hetzner.com/cloud/) | 比较服务器总成本与两段实际网络表现。 |
| 站点故障和模型调用证据 | [Uptime Kuma](https://github.com/louislam/uptime-kuma)、[Sentry](https://sentry.io/welcome/)、[Langfuse](https://langfuse.com/) | 分别补足基础服务存活、应用错误、LLM 调用跟踪与评估。上线采集前需确定脱敏与数据范围。 |
| 备份和恢复 | [restic](https://restic.net/) | 加密备份及恢复工具；需配合数据库一致性导出、定时执行、保留策略和恢复演练。 |
| 产品和交互发现 | [Hugging Face Spaces](https://huggingface.co/spaces)、[LobeHub](https://lobehub.com/) | 发现社区模型演示、Agent 工作台与团队协作产品交互。演示、源码许可与商业 API 供给是不同事项。 |

## 保存与使用

资源支持名称、功能介绍、关联模型、功能标签、来源、使用情况、个人备注、资料核对状态、核对日期与参考链接。原备注与功能介绍分开保存；修改使用情况不会自动变更资料核对结论。关联模型留空表示通用业务资源或其他模型，不代表支持所有模型。GPT、Claude 筛选同时保留通用资源。

输入名称、网址、功能、标签或备注时会切换到全库搜索；之后可继续缩小分组和分类。使用情况可筛选“待了解”“试用中”“使用中”。每次展示 12 项，可继续加载。保存条目后会切换到其分类、清除不兼容的筛选条件，并确保刚保存的条目可见。

“导出全部资源”下载当前已载入的完整列表，不受页面筛选和分页限制，包含备注、使用情况、参考信息和版本。导出仅在浏览器内生成文件。资源最多 100 项、总内容最多 256 KB；标签最多 8 个，每个最多 30 字。链接和参考链接只允许无内嵌凭证的 HTTP/HTTPS 地址；所有描述和备注以纯文本展示。浏览、编辑与保存不请求外部站点。

迁移 `257_quality_business_resources.sql` 一次性补入 28 个新入口（11 个用户提供、17 个扩展），与原有 14 项合计 42 项；OpenAI 状态页复用已有入口，仅补充缺失的原备注。按编号或已有链接去重，不覆盖管理员已有记录；已有 100 项或内容接近 256 KB 上限时，仅补入剩余容量允许的条目。管理员删除与编辑不会在迁移重跑时恢复或被覆盖。资料保存在已有管理员专用接口和独立版本控制下，不影响监控计划或题库。
