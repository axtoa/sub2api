<template>
  <AppLayout>
    <section class="relative h-[calc(100vh-7.5rem)] min-h-0 overflow-hidden rounded-lg border border-gray-200 bg-white shadow-sm dark:border-dark-700 dark:bg-dark-900">
      <div class="grid h-full min-h-0 grid-cols-1 lg:grid-cols-[minmax(0,1fr)_auto]">
        <div class="flex h-full min-h-0 min-w-0 flex-col">
          <div class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-200 px-4 py-3 dark:border-dark-700 sm:px-5">
            <div class="min-w-0">
              <p class="text-sm font-medium text-gray-900 dark:text-white">任务记录 {{ activeTab === 'image' ? `${batchJobs.length}/${imageRecordLimit}` : `${videoTasks.length}/${videoLimits.maxRecords}` }}</p>
              <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
                {{ activeTab === 'image' ? '图片任务统一保留 3 天，超过数量上限会优先清理最早记录。' : `视频任务保留 ${videoLimits.retentionDays} 天，进行中 ${videoRunningCount}/${videoLimits.maxRunning}。` }}
              </p>
            </div>
            <div class="flex items-center gap-2">
              <button type="button" class="btn btn-secondary btn-sm" :disabled="loadingKeys || loadingJobs || videoLoadingTasks" @click="refreshPage">
                <Icon name="refresh" size="sm" class="mr-1.5" :class="loadingKeys || loadingJobs || videoLoadingTasks ? 'animate-spin' : ''" />
                刷新
              </button>
              <button type="button" class="btn btn-secondary btn-sm" @click="showGuideModal = true">
                <Icon name="book" size="sm" class="mr-1.5" />
                使用说明
              </button>
            </div>
          </div>

          <div class="min-h-0 flex-1 overflow-y-auto bg-gray-50/70 px-4 py-4 dark:bg-dark-950/30 sm:px-5">
            <div v-if="activeTab === 'image'" class="grid gap-4 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
              <div
                v-for="job in recentImageJobs"
                :key="job.id"
                role="button"
                tabindex="0"
                class="group overflow-hidden rounded-lg border border-gray-200 bg-white text-left shadow-sm transition hover:-translate-y-0.5 hover:border-primary-200 hover:shadow-md dark:border-dark-700 dark:bg-dark-800 dark:hover:border-primary-700/60"
                @click="openJobPreview(job)"
                @keydown.enter.prevent="openJobPreview(job)"
                @keydown.space.prevent="openJobPreview(job)"
              >
                <div class="relative flex aspect-[4/3] items-center justify-center overflow-hidden bg-gradient-to-br from-sky-50 via-rose-50 to-emerald-50 dark:from-sky-950/30 dark:via-rose-950/20 dark:to-emerald-950/20">
                  <img
                    v-if="imageJobPreviewUrls[job.id]"
                    :src="imageJobPreviewUrls[job.id]"
                    :alt="job.task_name || '图片作品'"
                    class="h-full w-full object-cover transition duration-300 group-hover:scale-[1.02]"
                    @error="handleJobPreviewError(job.id)"
                  />
                  <Icon v-else-if="imageJobPreviewLoadingIds.has(job.id)" name="refresh" size="lg" class="animate-spin text-primary-500/80" />
                  <Icon v-else name="sparkles" size="xl" class="text-primary-500/80" />
                  <div class="absolute inset-x-2 top-2 flex justify-end gap-1 opacity-0 transition group-hover:opacity-100">
                    <button
                      type="button"
                      class="rounded-md bg-black/60 p-1.5 text-white shadow-sm transition hover:bg-black/80"
                      :title="t('batchImage.actions.viewDetail')"
                      @click.stop="selectJob(job.id)"
                    >
                      <Icon name="document" size="sm" />
                    </button>
                    <button
                      v-if="canDownload(displayJob(job))"
                      type="button"
                      class="rounded-md bg-black/60 p-1.5 text-white shadow-sm transition hover:bg-emerald-600"
                      :disabled="downloading"
                      :title="t('batchImage.actions.download')"
                      @click.stop="downloadJob(job)"
                    >
                      <Icon :name="isDownloadingJob(job.id) ? 'refresh' : 'download'" size="sm" :class="isDownloadingJob(job.id) ? 'animate-spin' : ''" />
                    </button>
                    <button
                      v-if="canDeleteRecord(displayJob(job))"
                      type="button"
                      class="rounded-md bg-black/60 p-1.5 text-white shadow-sm transition hover:bg-red-600"
                      :disabled="deletingBatchId === job.id"
                      :title="t('common.delete')"
                      @click.stop="deleteJob(job)"
                    >
                      <Icon :name="deletingBatchId === job.id ? 'refresh' : 'trash'" size="sm" :class="deletingBatchId === job.id ? 'animate-spin' : ''" />
                    </button>
                  </div>
                </div>
                <div class="space-y-2 p-3">
                  <div class="flex items-center justify-between gap-2">
                    <p class="min-w-0 truncate text-sm font-medium text-gray-900 dark:text-white">{{ job.task_name || defaultTaskName(job.created_at) }}</p>
                    <span class="badge flex-shrink-0" :class="statusBadgeClass(displayJob(job))">{{ statusLabel(displayJob(job)) }}</span>
                  </div>
                  <p class="truncate text-xs text-gray-500 dark:text-gray-400">{{ job.model }} · {{ formatDate(job.created_at) }}</p>
                  <div class="flex items-center justify-between text-xs text-gray-500 dark:text-gray-400">
                    <span>成功 {{ displayJob(job).success_count }} / {{ displayJob(job).item_count }}</span>
                    <span>{{ costLabel(displayJob(job)) }}</span>
                  </div>
                </div>
              </div>

              <div v-if="!loadingJobs && recentImageJobs.length === 0" class="col-span-full flex min-h-[420px] flex-col items-center justify-center rounded-lg border border-dashed border-gray-200 bg-white px-6 text-center dark:border-dark-700 dark:bg-dark-800">
                <Icon name="sparkles" size="xl" class="mb-4 h-12 w-12 text-primary-400" />
                <p class="text-base font-medium text-gray-900 dark:text-white">这里还没有作品</p>
                <p class="mt-2 max-w-md text-sm leading-6 text-gray-500 dark:text-gray-400">
                  选一个模板，或写下你想要的画面，我们一起把它变出来。
                </p>
              </div>
            </div>

            <div v-else class="space-y-4">
              <div v-if="videoLoadingTasks && !videoTasks.length" class="flex min-h-[420px] items-center justify-center text-sm text-gray-500 dark:text-gray-400">加载视频任务中...</div>
              <div v-else-if="!videoTasks.length" class="flex min-h-[420px] flex-col items-center justify-center rounded-lg border border-dashed border-gray-200 bg-white px-6 text-center dark:border-dark-700 dark:bg-dark-800">
                <Icon name="sparkles" size="xl" class="mb-4 h-12 w-12 text-primary-400" />
                <p class="text-base font-medium text-gray-900 dark:text-white">还没有视频作品</p>
                <p class="mt-2 max-w-md text-sm leading-6 text-gray-500 dark:text-gray-400">描述一个镜头、一个动作或一段氛围，第一条视频任务会在这里等你。</p>
              </div>
              <template v-else>
                <div class="flex flex-wrap items-center justify-between gap-3 text-xs text-gray-500 dark:text-gray-400">
                  <p>视频任务最多保留 <span class="font-medium text-red-500">{{ videoLimits.retentionDays }}</span> 天；超过 <span class="font-medium text-red-500">{{ videoLimits.maxRecords }}</span> 条会提前清理，完成后请及时下载。</p>
                  <p>任务记录 {{ videoTasks.length }}/{{ videoLimits.maxRecords }} · 进行中 {{ videoRunningCount }}/{{ videoLimits.maxRunning }}</p>
                </div>
                <div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
                  <button
                    v-for="task in videoTasks"
                    :key="task.id"
                    type="button"
                    class="group overflow-hidden rounded-lg border border-gray-200 bg-white text-left shadow-sm transition hover:-translate-y-0.5 hover:border-primary-200 hover:shadow-md dark:border-dark-700 dark:bg-dark-800 dark:hover:border-primary-700/60"
                    @click="openVideoDetail(task)"
                  >
                    <div class="relative aspect-[4/3] overflow-hidden bg-black">
                      <img
                        v-if="videoThumbnailUrls[task.id]"
                        :src="videoThumbnailUrls[task.id]"
                        :alt="task.prompt_preview || task.model"
                        class="h-full w-full object-cover transition duration-300 group-hover:scale-[1.02]"
                      />
                      <div v-else class="flex h-full w-full items-center justify-center bg-gradient-to-br from-slate-950 via-slate-900 to-emerald-950">
                        <Icon v-if="videoThumbnailLoadingIds.has(task.id)" name="refresh" size="lg" class="animate-spin text-white/80" />
                        <Icon v-else name="sparkles" size="xl" class="text-white/80" />
                      </div>
                      <div class="absolute inset-x-0 bottom-0 bg-gradient-to-t from-black/80 via-black/35 to-transparent p-3 pt-10">
                        <p class="line-clamp-2 text-sm font-medium leading-5 text-white">{{ task.prompt_preview || '无提示词' }}</p>
                      </div>
                      <div class="absolute left-2 top-2 rounded bg-black/50 px-2 py-1 text-xs font-medium text-white" :title="videoExpiryHint(task)">
                        {{ videoRetentionLabel(task) }}
                      </div>
                      <span class="absolute right-2 top-2 rounded-full px-2 py-1 text-xs font-medium shadow-sm" :class="creativeVideoPillClass(task.status)">
                        {{ creativeVideoStatusLabel(task.status) }}
                      </span>
                      <div class="absolute right-2 top-10 flex gap-1 opacity-0 transition group-hover:opacity-100">
                        <button type="button" class="rounded-md bg-black/55 p-1.5 text-white hover:bg-black/75" title="再次生成" @click.stop="reuseVideoTask(task)">
                          <Icon name="refresh" size="sm" />
                        </button>
                        <button v-if="isCreativeVideoCompleted(task.status)" type="button" class="rounded-md bg-black/55 p-1.5 text-white hover:bg-black/75" :disabled="videoDownloadingId === task.id" title="下载视频" @click.stop="downloadVideoTask(task)">
                          <Icon :name="videoDownloadingId === task.id ? 'refresh' : 'download'" size="sm" :class="videoDownloadingId === task.id ? 'animate-spin' : ''" />
                        </button>
                        <button v-if="isCreativeVideoTerminal(task.status)" type="button" class="rounded-md bg-black/55 p-1.5 text-white hover:bg-red-600" :disabled="videoDeletingId === task.id" title="删除记录" @click.stop="removeVideoTask(task)">
                          <Icon :name="videoDeletingId === task.id ? 'refresh' : 'trash'" size="sm" :class="videoDeletingId === task.id ? 'animate-spin' : ''" />
                        </button>
                      </div>
                    </div>
                    <div class="space-y-2 p-3">
                      <div class="flex items-center justify-between gap-2 text-xs text-gray-500 dark:text-gray-400">
                        <span class="min-w-0 truncate text-gray-700 dark:text-gray-300">{{ task.model }}</span>
                        <span class="flex-shrink-0">{{ shortDateTime(task.created_at) }}</span>
                      </div>
                      <p v-if="creativeVideoElapsedText(task)" class="text-xs text-amber-600 dark:text-amber-300">{{ creativeVideoElapsedText(task) }}</p>
                      <p v-else class="text-xs text-gray-500 dark:text-gray-400">{{ task.resolution || '720p' }} · {{ task.duration_seconds || videoForm.duration }} 秒</p>
                    </div>
                  </button>
                </div>
              </template>
            </div>
          </div>

          <div class="flex-shrink-0 border-t border-gray-200 bg-white p-3 dark:border-dark-700 dark:bg-dark-900 sm:p-4">
            <form class="rounded-lg border border-gray-200 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-800/70" @submit.prevent="submitCreative">
              <div class="mb-3 flex flex-wrap items-center justify-between gap-2">
                <div class="inline-flex rounded-lg bg-white p-1 shadow-sm ring-1 ring-gray-200 dark:bg-dark-900 dark:ring-dark-700">
                  <button type="button" class="rounded-md px-3 py-1.5 text-sm font-medium transition" :class="activeTab === 'image' ? 'bg-primary-600 text-white shadow-sm' : 'text-gray-600 hover:text-gray-900 dark:text-gray-300 dark:hover:text-white'" @click="switchCreativeMode('image')">图片</button>
                  <button type="button" class="rounded-md px-3 py-1.5 text-sm font-medium transition" :class="activeTab === 'video' ? 'bg-primary-600 text-white shadow-sm' : 'text-gray-600 hover:text-gray-900 dark:text-gray-300 dark:hover:text-white'" @click="switchCreativeMode('video')">视频</button>
                </div>
                <div v-if="activeTab === 'image'" class="inline-flex rounded-lg bg-white p-1 shadow-sm ring-1 ring-gray-200 dark:bg-dark-900 dark:ring-dark-700">
                  <button type="button" class="rounded-md px-3 py-1.5 text-sm font-medium transition" :class="imageTool === 'text' ? 'bg-gray-900 text-white dark:bg-white dark:text-gray-900' : 'text-gray-600 hover:text-gray-900 dark:text-gray-300 dark:hover:text-white'" @click="imageTool = 'text'">文生图</button>
                  <button type="button" class="rounded-md px-3 py-1.5 text-sm font-medium transition" :class="imageTool === 'edit' ? 'bg-gray-900 text-white dark:bg-white dark:text-gray-900' : 'text-gray-600 hover:text-gray-900 dark:text-gray-300 dark:hover:text-white'" @click="imageTool = 'edit'">改图</button>
                </div>
                <div v-else class="flex items-center gap-2">
                  <button type="button" class="inline-flex items-center rounded-md px-2.5 py-1.5 text-xs font-medium text-gray-500 transition hover:bg-white hover:text-gray-900 dark:text-gray-400 dark:hover:bg-dark-900 dark:hover:text-white" @click="showVideoApiDocsModal = true">
                    <Icon name="document" size="xs" class="mr-1.5" />
                    API 文档
                  </button>
                  <button type="button" class="inline-flex items-center rounded-md px-2.5 py-1.5 text-xs font-medium text-gray-500 transition hover:bg-white hover:text-gray-900 dark:text-gray-400 dark:hover:bg-dark-900 dark:hover:text-white" @click="showVideoPromptGuideModal = true">
                    <Icon name="book" size="xs" class="mr-1.5" />
                    提示词规范
                  </button>
                </div>
              </div>

              <div class="mb-3 grid gap-2 md:grid-cols-[minmax(180px,1.2fr)_120px_120px_112px]">
                <button type="button" class="input flex items-center justify-between gap-2 bg-white text-left dark:bg-dark-900" @click="showModelPicker = true">
                  <span class="min-w-0">
                    <span class="block text-xs text-gray-400">模型选择</span>
                    <span class="block truncate text-sm font-medium text-gray-900 dark:text-white">{{ selectedModelLabel }}</span>
                  </span>
                  <Icon name="chevronDown" size="sm" class="text-gray-400" />
                </button>
                <select v-model="composerAspectRatio" class="input bg-white dark:bg-dark-900">
                  <option v-for="ratio in aspectRatioOptions" :key="ratio" :value="ratio">{{ ratio }}</option>
                </select>
                <select v-model="composerSize" class="input bg-white dark:bg-dark-900">
                  <option v-for="size in currentSizeOptions" :key="size" :value="size">{{ size }}</option>
                </select>
                <select v-if="activeTab === 'image'" v-model.number="composerCount" class="input bg-white dark:bg-dark-900">
                  <option v-for="count in currentCountOptions" :key="count" :value="count">{{ activeTab === 'image' ? `${count}张` : `${count}条` }}</option>
                </select>
                <select v-else v-model.number="videoForm.duration" class="input bg-white dark:bg-dark-900">
                  <option v-for="seconds in videoDurationOptions" :key="seconds" :value="seconds">{{ seconds }} 秒</option>
                </select>
              </div>

              <div v-if="activeTab === 'image' && imageTool === 'edit'" class="mb-3">
                <label class="flex min-h-[76px] cursor-pointer items-center justify-center rounded-lg border border-dashed border-gray-300 bg-white px-3 py-3 text-sm text-gray-500 transition hover:border-primary-300 hover:text-primary-600 dark:border-dark-600 dark:bg-dark-900 dark:text-gray-400 dark:hover:border-primary-700 dark:hover:text-primary-300">
                  <Icon name="upload" size="sm" class="mr-2" />
                  {{ referenceImageDrafts.length ? `已选择 ${referenceImageDrafts.length} 张参考图` : '上传参考图后再告诉我想怎么改' }}
                  <input type="file" accept="image/png,image/jpeg,image/webp" class="hidden" @change="handleSingleEditReferenceImage" />
                </label>
              </div>

              <div v-if="activeTab === 'video'" class="mb-3">
                <div v-if="videoFrameDraft" class="flex items-center justify-between gap-3 rounded-lg border border-gray-200 bg-white px-3 py-2 dark:border-dark-700 dark:bg-dark-900">
                  <div class="min-w-0">
                    <p class="truncate text-sm font-medium text-gray-900 dark:text-white">{{ videoFrameDraft.name }}</p>
                    <p class="text-xs text-gray-500 dark:text-gray-400">作为视频首帧参考，会随本次任务提交</p>
                  </div>
                  <button type="button" class="btn btn-secondary btn-sm" @click="videoFrameDraft = null">移除</button>
                </div>
                <label v-else class="flex min-h-[64px] cursor-pointer items-center justify-center rounded-lg border border-dashed border-gray-300 bg-white px-3 py-3 text-sm text-gray-500 transition hover:border-primary-300 hover:text-primary-600 dark:border-dark-600 dark:bg-dark-900 dark:text-gray-400 dark:hover:border-primary-700 dark:hover:text-primary-300">
                  <Icon name="upload" size="sm" class="mr-2" />
                  上传首帧图（可选）
                  <input type="file" accept="image/png,image/jpeg,image/webp" class="hidden" @change="handleVideoFrameImage" />
                </label>
              </div>

              <textarea
                v-model="creativePrompt"
                rows="3"
                class="w-full resize-y rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm leading-6 outline-none transition focus:border-primary-500 focus:ring-2 focus:ring-primary-100 dark:border-dark-600 dark:bg-dark-900 dark:text-gray-100 dark:focus:border-primary-500 dark:focus:ring-primary-900/40"
                :placeholder="creativePromptPlaceholder"
              />
              <div class="mt-3 flex flex-wrap items-center justify-between gap-3">
                <p class="text-xs text-gray-500 dark:text-gray-400">{{ creativeComposerHint }}</p>
                <button type="submit" class="btn btn-primary min-w-[128px] justify-center" :disabled="creativeSubmittingDisabled">
                  <Icon v-if="submitting || videoSubmitting" name="refresh" size="sm" class="mr-2 animate-spin" />
                  {{ submitting || videoSubmitting ? '提交中...' : activeTab === 'image' ? '生成图片' : '生成视频' }}
                </button>
              </div>
            </form>
          </div>
        </div>

        <aside
          v-if="activeTab === 'image'"
          class="relative h-full min-h-0 border-l border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-900"
          :class="templateDrawerState === 'expanded' ? 'absolute inset-y-0 right-0 z-20 w-full lg:w-[calc(100%-0px)]' : templateDrawerState === 'rail' ? 'w-[280px]' : 'w-12'"
        >
          <button
            type="button"
            class="absolute -left-3 top-1/2 z-30 flex h-20 w-6 -translate-y-1/2 items-center justify-center rounded-full border border-gray-200 bg-white text-gray-500 shadow-sm hover:text-primary-600 dark:border-dark-700 dark:bg-dark-800 dark:text-gray-300"
            @click="cycleTemplateDrawer"
          >
            <Icon :name="templateDrawerState === 'collapsed' ? 'chevronLeft' : 'chevronRight'" size="xs" />
          </button>
          <div v-if="templateDrawerState === 'collapsed'" class="flex h-full items-center justify-center">
            <button type="button" class="vertical-rl text-sm font-medium tracking-normal text-gray-500 dark:text-gray-400" @click="templateDrawerState = 'rail'">图片模板</button>
          </div>
          <div v-else class="flex h-full min-h-0 flex-col">
            <div class="flex flex-shrink-0 items-center justify-between gap-2 border-b border-gray-200 px-4 py-3 dark:border-dark-700">
              <div>
                <p class="text-sm font-semibold text-gray-900 dark:text-white">图片模板</p>
                <p class="text-xs text-gray-500 dark:text-gray-400">点击模板会带入提示词和参数</p>
              </div>
              <button type="button" class="btn btn-secondary btn-sm" @click="templateDrawerState = templateDrawerState === 'expanded' ? 'rail' : 'expanded'">
                {{ templateDrawerState === 'expanded' ? '收起全部' : '查看全部' }}
              </button>
            </div>
            <div class="flex flex-shrink-0 flex-wrap gap-2 border-b border-gray-100 p-3 dark:border-dark-800">
              <button v-for="category in templateCategories" :key="category" type="button" class="rounded-full px-3 py-1 text-xs font-medium transition" :class="templateCategory === category ? 'bg-gray-900 text-white dark:bg-white dark:text-gray-900' : 'bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-dark-800 dark:text-gray-300 dark:hover:bg-dark-700'" @click="templateCategory = category">{{ category }}</button>
            </div>
            <div class="min-h-0 flex-1 overflow-y-auto p-3">
              <div :class="templateDrawerState === 'expanded' ? 'columns-1 gap-3 sm:columns-2 xl:columns-3 2xl:columns-4' : 'space-y-3'">
                <button
                  v-for="template in filteredCreativeTemplates"
                  :key="template.id"
                  type="button"
                  class="mb-3 w-full break-inside-avoid overflow-hidden rounded-lg border border-gray-200 bg-white text-left shadow-sm transition hover:-translate-y-0.5 hover:border-primary-200 hover:shadow-md dark:border-dark-700 dark:bg-dark-800 dark:hover:border-primary-700/60"
                  @click="applyCreativeTemplate(template)"
                >
                  <div class="relative aspect-[4/3] overflow-hidden bg-cover bg-center" :class="template.previewClass">
                    <div class="absolute inset-0 template-preview-sheen" />
                    <div class="absolute left-3 top-3 rounded-full bg-white/75 px-2 py-0.5 text-[11px] font-medium text-gray-700 shadow-sm backdrop-blur dark:bg-dark-900/70 dark:text-gray-200">
                      {{ template.mode === 'text' ? '文生图' : '改图' }}
                    </div>
                  </div>
                  <div class="space-y-2 p-3">
                    <div class="flex items-center justify-between gap-2">
                      <p class="truncate text-sm font-medium text-gray-900 dark:text-white">{{ template.title }}</p>
                      <span class="rounded-full bg-gray-100 px-2 py-0.5 text-xs text-gray-500 dark:bg-dark-700 dark:text-gray-300">{{ template.mode === 'text' ? '文生图' : '改图' }}</span>
                    </div>
                    <p class="line-clamp-2 text-xs leading-5 text-gray-500 dark:text-gray-400">{{ template.description }}</p>
                  </div>
                </button>
              </div>
            </div>
          </div>
        </aside>
      </div>
    </section>

    <TablePageLayout v-if="false && activeTab === 'image'">
      <template #filters>
        <div class="flex flex-col gap-3">
          <div class="flex flex-col gap-3 2xl:flex-row 2xl:items-center 2xl:justify-between">
            <div class="grid w-full grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-[260px_160px_144px_152px] 2xl:w-auto">
              <div class="min-w-0">
                <SearchInput
                  v-model="filters.taskName"
                  :placeholder="t('batchImage.filters.searchTaskName')"
                  class="w-full"
                  @search="applyFilters"
                />
              </div>
              <Select v-model="filters.apiKeyId" :options="apiKeyFilterOptions" class="w-full" @change="applyFilters" />
              <Select v-model="filters.status" :options="statusFilterOptions" class="w-full" @change="applyFilters" />
              <Select v-model="filters.downloaded" :options="downloadFilterOptions" class="w-full" @change="applyFilters" />
            </div>
            <div class="flex flex-wrap items-center justify-start gap-2 sm:justify-end 2xl:flex-shrink-0">
              <button type="button" class="btn btn-secondary" :disabled="loadingJobs" @click="resetFilters">
                {{ t('common.reset') }}
              </button>
              <button type="button" class="btn btn-secondary" :disabled="loadingKeys || loadingJobs" :title="t('common.refresh')" @click="refreshPage">
                <Icon name="refresh" size="md" :class="loadingKeys || loadingJobs ? 'animate-spin' : ''" />
              </button>
              <button type="button" class="btn btn-secondary" @click="showGuideModal = true">
                <Icon name="book" size="md" class="mr-2" />
                {{ t('batchImage.actions.usageGuide') }}
              </button>
              <button type="button" class="btn btn-primary" @click="openCreateModal">
                <Icon name="plus" size="md" class="mr-2" />
                {{ t('batchImage.actions.createJob') }}
              </button>
            </div>
          </div>

          <div
            v-if="selectedJobIds.size"
            class="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-gray-200 bg-white px-3 py-2 shadow-sm dark:border-dark-700 dark:bg-dark-800"
          >
            <i18n-t
              keypath="batchImage.list.selectedJobs"
              tag="span"
              scope="global"
              :plural="selectedJobIds.size"
              class="text-sm text-gray-600 dark:text-gray-300"
            >
              <template #count>
                <span class="font-medium text-gray-900 dark:text-white">{{ selectedJobIds.size }}</span>
              </template>
            </i18n-t>
            <div class="flex flex-wrap items-center gap-2">
              <button
                type="button"
                class="btn btn-secondary btn-sm"
                :disabled="bulkDownloading || selectedDownloadableRows.length === 0"
                @click="downloadSelectedJobs"
              >
                <Icon :name="bulkDownloading ? 'refresh' : 'download'" size="sm" class="mr-1.5" :class="bulkDownloading ? 'animate-spin' : ''" />
                {{ t('batchImage.actions.downloadSelected') }}
              </button>
              <button
                type="button"
                class="btn btn-secondary btn-sm text-red-600 hover:text-red-700 dark:text-red-400"
                :disabled="bulkDeleting"
                @click="deleteSelectedJobs"
              >
                <Icon :name="bulkDeleting ? 'refresh' : 'trash'" size="sm" class="mr-1.5" :class="bulkDeleting ? 'animate-spin' : ''" />
                {{ t('batchImage.actions.deleteRecords') }}
              </button>
            </div>
          </div>
        </div>
      </template>

      <template #table>
        <DataTable
          :columns="columns"
          :data="visibleBatchJobs"
          :loading="loadingKeys || loadingJobs"
          :expandable-actions="false"
          row-key="id"
        >
          <template #header-select>
            <input
              type="checkbox"
              class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
              :checked="allVisibleSelected"
              :indeterminate="someVisibleSelected"
              @change="toggleAllVisible(($event.target as HTMLInputElement).checked)"
            />
          </template>

          <template #cell-select="{ row }">
            <input
              type="checkbox"
              class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
              :checked="selectedJobIds.has(row.id)"
              @change="toggleJobSelection(row.id, ($event.target as HTMLInputElement).checked)"
              @click.stop
            />
          </template>

          <template #cell-id="{ row }">
	            <div class="flex w-[220px] items-start gap-1" :class="row.is_child ? 'pl-6' : ''">
	              <button
	                v-if="row.child_count > 0 && !row.is_child"
	                type="button"
	                class="mt-1 flex h-6 w-6 flex-shrink-0 items-center justify-center rounded-md text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-900 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/30 dark:text-gray-400 dark:hover:bg-dark-700 dark:hover:text-white"
	                :title="expandedParentIds.has(row.id) ? t('batchImage.list.collapseChildren') : t('batchImage.list.expandChildren', { n: row.child_count }, row.child_count)"
	                @click.stop="toggleChildRows(row.id)"
	              >
	                <Icon :name="expandedParentIds.has(row.id) ? 'chevronDown' : 'chevronRight'" size="xs" />
	              </button>
	              <span v-else class="w-6 flex-shrink-0" />
	              <button type="button" class="min-w-0 flex-1 rounded-lg py-1 text-left transition-colors hover:bg-gray-100 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/30 dark:hover:bg-dark-700" @click="selectJob(row.id)">
	                <span
	                  class="flex min-w-0 items-center gap-2 text-sm font-medium"
	                  :class="row.task_name ? 'text-gray-900 dark:text-white' : 'text-gray-500 dark:text-gray-400'"
                >
                  <span class="min-w-0 truncate">{{ row.task_name || defaultTaskName(row.created_at) }}</span>
                  <span v-if="row.child_count > 0 && !row.is_child" class="flex-shrink-0 rounded-full bg-gray-100 px-2 py-0.5 text-xs font-normal text-gray-600 dark:bg-dark-700 dark:text-gray-300">
                    {{ t('batchImage.list.childCount', { n: row.child_count }, row.child_count) }}
                  </span>
                  <span v-if="row.is_child" class="flex-shrink-0 rounded-full bg-amber-50 px-2 py-0.5 text-xs font-normal text-amber-700 dark:bg-amber-900/20 dark:text-amber-300">
                    {{ t('batchImage.list.childBadge') }}
                  </span>
	                </span>
	                <span class="mt-1 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-gray-500 dark:text-gray-400">
	                  <span>{{ formatDate(row.created_at) }}</span>
	                </span>
	              </button>
	            </div>
	          </template>

          <template #cell-model="{ row }">
	            <div class="mx-auto max-w-[180px] text-center">
	              <p class="truncate text-sm text-gray-700 dark:text-gray-300" :title="row.model">{{ row.model }}</p>
	            </div>
	          </template>

          <template #cell-api_key_name="{ value }">
            <span class="block truncate text-center text-sm text-gray-700 dark:text-gray-300">
              {{ value || t('batchImage.list.keyNotRecorded') }}
            </span>
          </template>

          <template #cell-status="{ row }">
            <div class="flex justify-center">
              <span :class="statusBadgeClass(displayJob(row))" class="badge">
                {{ statusLabel(displayJob(row)) }}
              </span>
            </div>
          </template>

          <template #cell-counts="{ row }">
            <div class="flex items-center justify-center gap-2 text-sm tabular-nums">
              <span class="text-emerald-600 dark:text-emerald-300">{{ displayJob(row).success_count }}</span>
              <span class="text-gray-300 dark:text-dark-500">/</span>
              <span :class="displayJob(row).fail_count > 0 ? 'text-red-600 dark:text-red-300' : 'text-gray-400 dark:text-gray-500'">{{ displayJob(row).fail_count }}</span>
              <span class="text-xs text-gray-400 dark:text-gray-500">{{ t('batchImage.list.totalCount', { n: displayJob(row).item_count }) }}</span>
            </div>
          </template>

          <template #cell-cost="{ row }">
            <span class="block text-center text-sm text-gray-700 dark:text-gray-300">
              {{ costLabel(displayJob(row)) }}
            </span>
          </template>

          <template #cell-downloaded="{ row }">
            <span class="block text-center text-sm" :class="row.downloaded_at ? 'text-emerald-700 dark:text-emerald-300' : 'text-gray-500 dark:text-gray-400'">
              {{ row.downloaded_at ? formatDate(row.downloaded_at) : t('batchImage.list.notDownloaded') }}
            </span>
          </template>

	          <template #cell-actions="{ row }">
	            <div class="flex items-center justify-center gap-1">
              <button
                type="button"
                class="batch-row-action flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-gray-100 hover:text-primary-600 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/30 dark:hover:bg-dark-700 dark:hover:text-primary-400"
                :title="t('batchImage.actions.viewDetail')"
                @click="selectJob(row.id)"
              >
                <Icon name="eye" size="sm" />
                <span class="text-xs">{{ t('common.view') }}</span>
              </button>
              <button
                type="button"
                class="batch-row-action flex flex-col items-center gap-0.5 rounded-lg p-1.5 transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/30"
                :class="canDownload(row) ? 'text-gray-500 hover:bg-green-50 hover:text-green-600 dark:hover:bg-green-900/20 dark:hover:text-green-400' : 'text-gray-300 dark:text-dark-500'"
                :disabled="!canDownload(row) || downloading"
                :title="t('batchImage.actions.downloadZip')"
                @click="downloadJob(row)"
              >
                <Icon
                  :name="isDownloadingJob(row.id) ? 'refresh' : 'download'"
	                  size="sm"
	                  :class="isDownloadingJob(row.id) ? 'animate-spin' : ''"
	                />
                <span class="text-xs">{{ t('batchImage.actions.download') }}</span>
	              </button>
              <div v-if="canRetry(row) || canDeleteRecord(row)">
                <button
                  type="button"
                  class="batch-row-action flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-900 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/30 dark:hover:bg-dark-700 dark:hover:text-white"
                  :class="{ 'bg-gray-100 text-gray-900 dark:bg-dark-700 dark:text-white': openMoreJobId === row.id }"
                  :title="t('batchImage.actions.moreActions')"
                  @click.stop="toggleMoreMenu(row, $event)"
                >
                  <Icon name="more" size="sm" />
                  <span class="text-xs">{{ t('common.more') }}</span>
                </button>
              </div>
	            </div>
	          </template>

          <template #empty>
            <div class="flex min-h-[260px] flex-col items-center justify-center py-6 md:min-h-[300px]">
              <Icon name="sparkles" size="xl" class="mb-4 h-12 w-12 text-gray-400 dark:text-dark-500" />
              <p class="text-lg font-medium text-gray-900 dark:text-gray-100">{{ t('batchImage.list.empty') }}</p>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                {{ t('batchImage.list.emptyHint') }}
              </p>
            </div>
          </template>
        </DataTable>
      </template>

      <template #pagination>
        <div
          v-if="visibleBatchJobs.length > 0 || pagination.page > 1"
          class="flex flex-col gap-3 border-t border-gray-200 bg-white px-4 py-3 dark:border-dark-700 dark:bg-dark-800 sm:flex-row sm:items-center sm:justify-between sm:px-6"
        >
          <div class="flex flex-wrap items-center gap-3 text-sm text-gray-700 dark:text-gray-300">
            <i18n-t keypath="batchImage.pagination.pageNumber" tag="span" scope="global">
              <template #page>
                <span class="font-medium">{{ pagination.page }}</span>
              </template>
            </i18n-t>
            <i18n-t keypath="batchImage.pagination.pageItems" tag="span" scope="global">
              <template #count>
                <span class="font-medium">{{ visibleBatchJobs.length }}</span>
              </template>
            </i18n-t>
            <div class="flex items-center gap-2">
              <span>{{ t('pagination.perPage') }}</span>
              <Select
                v-model="pagination.page_size"
                :options="batchPageSizeOptions"
                class="w-24"
                @change="handlePageSizeChange"
              />
            </div>
          </div>
          <div class="flex items-center justify-end gap-2">
            <button
              type="button"
              class="btn btn-secondary btn-sm"
              :disabled="pagination.page <= 1 || loadingJobs"
              @click="handlePageChange(pagination.page - 1)"
            >
              <Icon name="chevronLeft" size="sm" class="mr-1" />
              {{ t('pagination.previous') }}
            </button>
            <button
              type="button"
              class="btn btn-secondary btn-sm"
              :disabled="!pagination.has_more || loadingJobs"
              @click="handlePageChange(pagination.page + 1)"
            >
              {{ t('pagination.next') }}
              <Icon name="chevronRight" size="sm" class="ml-1" />
            </button>
          </div>
        </div>
      </template>
    </TablePageLayout>

    <BaseDialog :show="!!videoDetailTask" title="视频详情" width="extra-wide" :z-index="60" @close="closeVideoPreview">
      <div v-if="videoDetailTask" class="grid gap-5 lg:grid-cols-[minmax(0,1.6fr)_minmax(280px,0.8fr)]">
        <div class="relative flex min-h-[520px] items-center justify-center rounded-lg bg-black p-3 lg:min-h-[620px]">
          <video
            v-if="videoPreviewUrl"
            :src="videoPreviewUrl"
            :poster="videoDetailTask ? videoThumbnailUrls[videoDetailTask.id] : undefined"
            class="h-full max-h-[78vh] w-full rounded-md object-contain"
            controls
            autoplay
            :muted="true"
            playsinline
            preload="auto"
            @error="handleVideoPreviewError"
            @loadeddata="videoPreviewError = ''"
          />
          <div v-if="videoPreviewError" class="absolute inset-3 flex flex-col items-center justify-center gap-3 rounded-md bg-black/80 px-6 text-center text-sm text-white/80">
            <Icon name="exclamationTriangle" size="lg" class="text-amber-300" />
            <p>{{ videoPreviewError }}</p>
            <div class="flex flex-wrap justify-center gap-2">
              <button v-if="videoDetailTask" type="button" class="btn btn-secondary btn-sm" @click="previewVideoTask(videoDetailTask)">
                <Icon name="refresh" size="sm" class="mr-1.5" />
                重新加载
              </button>
              <button v-if="videoDetailTask" type="button" class="btn btn-primary btn-sm" @click="downloadVideoTask(videoDetailTask)">
                <Icon name="download" size="sm" class="mr-1.5" />
                下载播放
              </button>
            </div>
          </div>
          <div v-else-if="!videoPreviewUrl" class="flex flex-col items-center gap-3 text-sm text-white/70">
            <Icon :name="isCreativeVideoProcessing(videoDetailTask.status) ? 'refresh' : 'sparkles'" size="lg" :class="isCreativeVideoProcessing(videoDetailTask.status) ? 'animate-spin' : ''" />
            {{ isCreativeVideoProcessing(videoDetailTask.status) ? '视频还在生成中' : '视频暂不可预览' }}
          </div>
        </div>
        <div class="space-y-5">
          <div>
            <span class="inline-flex items-center rounded-full px-2.5 py-1 text-xs font-medium" :class="creativeVideoPillClass(videoDetailTask.status)">
              {{ creativeVideoStatusLabel(videoDetailTask.status) }}
            </span>
          </div>
          <div class="space-y-2">
            <div class="flex items-center justify-between gap-2">
              <h3 class="text-sm font-semibold text-gray-900 dark:text-white">提示词</h3>
              <button type="button" class="btn-ghost btn-icon" title="复制提示词" @click="copyVideoPrompt(videoDetailTask)">
                <Icon name="copy" size="sm" />
              </button>
            </div>
            <p class="rounded-lg bg-gray-50 p-3 text-sm leading-6 text-gray-700 dark:bg-dark-800 dark:text-gray-300">{{ videoDetailTask.prompt_preview || '无提示词' }}</p>
          </div>
          <p
            v-if="creativeVideoProgressHint(videoDetailTask)"
            class="rounded-md px-3 py-2 text-xs leading-5"
            :class="creativeVideoProgressHintClass(videoDetailTask)"
          >
            {{ creativeVideoProgressHint(videoDetailTask) }}
          </p>
          <dl class="grid grid-cols-2 gap-x-4 gap-y-3 text-sm">
            <dt class="text-gray-500 dark:text-gray-400">生成模型</dt>
            <dd class="text-right text-gray-900 dark:text-white">{{ videoDetailTask.model || '-' }}</dd>
            <dt class="text-gray-500 dark:text-gray-400">尺寸</dt>
            <dd class="text-right text-gray-900 dark:text-white">{{ videoDetailAspectRatio }}</dd>
            <dt class="text-gray-500 dark:text-gray-400">分辨率</dt>
            <dd class="text-right text-gray-900 dark:text-white">{{ videoDetailTask.resolution || '-' }}</dd>
            <dt class="text-gray-500 dark:text-gray-400">时长</dt>
            <dd class="text-right text-gray-900 dark:text-white">{{ videoDetailTask.duration_seconds || videoForm.duration }} 秒</dd>
            <dt class="text-gray-500 dark:text-gray-400">创建时间</dt>
            <dd class="text-right text-gray-900 dark:text-white">{{ formatDate(videoDetailTask.created_at) }}</dd>
            <dt class="text-gray-500 dark:text-gray-400">完成时间</dt>
            <dd class="text-right text-gray-900 dark:text-white">{{ videoDetailTask.completed_at ? formatDate(videoDetailTask.completed_at) : '-' }}</dd>
            <dt class="text-gray-500 dark:text-gray-400">文件大小</dt>
            <dd class="text-right text-gray-900 dark:text-white">{{ formatVideoFileSize(videoDetailTask.file_size_bytes) }}</dd>
            <dt class="text-gray-500 dark:text-gray-400">已记录费用</dt>
            <dd class="text-right text-gray-900 dark:text-white">{{ formatVideoCost(videoDetailTask.actual_cost) }}</dd>
            <dt class="text-gray-500 dark:text-gray-400">生成耗时</dt>
            <dd class="text-right text-gray-900 dark:text-white">{{ formatVideoElapsed(videoDetailTask) }}</dd>
            <dt class="text-gray-500 dark:text-gray-400">到期时间</dt>
            <dd class="text-right text-gray-900 dark:text-white">{{ videoExpiresAtText(videoDetailTask) }}</dd>
          </dl>
          <div class="flex flex-wrap justify-end gap-2 border-t border-gray-200 pt-4 dark:border-dark-700">
            <button type="button" class="btn btn-secondary" @click="reuseVideoTask(videoDetailTask)">
              <Icon name="refresh" size="sm" class="mr-2" />
              再次生成
            </button>
            <button type="button" class="btn btn-primary" :disabled="!isCreativeVideoCompleted(videoDetailTask.status) || videoDownloadingId === videoDetailTask.id" @click="downloadVideoTask(videoDetailTask)">
              <Icon :name="videoDownloadingId === videoDetailTask.id ? 'refresh' : 'download'" size="sm" class="mr-2" :class="videoDownloadingId === videoDetailTask.id ? 'animate-spin' : ''" />
              下载视频
            </button>
            <button v-if="isCreativeVideoTerminal(videoDetailTask.status)" type="button" class="btn-ghost btn-icon text-red-600 hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-900/20" title="删除记录" @click="removeVideoTask(videoDetailTask)">
              <Icon name="trash" size="sm" />
            </button>
          </div>
        </div>
      </div>
    </BaseDialog>

    <BaseDialog :show="showModelPicker" title="模型选择" width="wide" :z-index="70" @close="showModelPicker = false">
      <div class="space-y-3">
        <div class="rounded-lg border border-blue-100 bg-blue-50 px-3 py-2 text-xs leading-5 text-blue-800 dark:border-blue-900/60 dark:bg-blue-950/30 dark:text-blue-100">
          这里会展示你拥有的所有 API Key；能否用于当前创作模式，由平台能力接入状态和分组开关自动判断。
        </div>
        <div v-if="currentModelChoices.length === 0" class="rounded-lg border border-dashed border-gray-200 px-4 py-10 text-center dark:border-dark-700">
          <Icon name="sparkles" size="lg" class="mx-auto mb-3 text-gray-400" />
          <p class="text-sm font-medium text-gray-900 dark:text-white">{{ activeTab === 'image' ? '还没有可用于图片创作的模型' : '还没有可用于视频创作的模型' }}</p>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">配置好可用的 API Key 后，这里就可以开始生成作品。</p>
        </div>
        <template v-else>
          <button
            v-for="choice in currentModelChoices"
            :key="choice.id"
            type="button"
            class="flex w-full items-center justify-between gap-3 rounded-lg border border-gray-200 px-4 py-3 text-left transition dark:border-dark-700"
            :class="choice.disabled ? 'cursor-not-allowed bg-gray-50 opacity-70 dark:bg-dark-900' : choice.selected ? 'border-primary-300 bg-primary-50 hover:border-primary-300 dark:border-primary-700 dark:bg-primary-950/30' : 'bg-white hover:border-primary-200 hover:bg-primary-50/40 dark:bg-dark-800 dark:hover:border-primary-700/60 dark:hover:bg-primary-950/20'"
            :disabled="choice.disabled"
            @click="selectModelChoice(choice)"
          >
            <span class="min-w-0">
              <span class="block truncate text-sm font-medium text-gray-900 dark:text-white">{{ choice.label }}</span>
              <span class="mt-1 block truncate text-xs text-gray-500 dark:text-gray-400">{{ choice.platformLabel }} · {{ choice.maskedKey }}</span>
              <span v-if="choice.reason" class="mt-1 block truncate text-xs text-amber-600 dark:text-amber-400">{{ choice.reason }}</span>
            </span>
            <span class="flex-shrink-0 rounded-full px-2.5 py-1 text-xs font-medium" :class="choice.selected ? 'bg-primary-600 text-white' : 'bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-gray-300'">
              {{ choice.selected ? '当前' : choice.disabled ? '不可用' : '切换' }}
            </span>
          </button>
        </template>
      </div>
    </BaseDialog>

    <BaseDialog :show="showVideoApiDocsModal" title="视频 API 文档" width="extra-wide" :z-index="80" @close="showVideoApiDocsModal = false">
      <div class="max-h-[72vh] space-y-6 overflow-y-auto pr-2 text-sm leading-6 text-gray-700 dark:text-gray-200">
        <section class="space-y-3">
          <p>视频生成采用异步任务方式：提交任务后保存 <code class="rounded bg-gray-100 px-1 py-0.5 text-xs dark:bg-dark-800">request_id</code>，随后查询状态并下载结果。创建、查询和下载必须使用同一个 API Key。</p>
          <div class="rounded-lg border border-blue-100 bg-blue-50 px-3 py-2 text-xs leading-5 text-blue-800 dark:border-blue-900/60 dark:bg-blue-950/30 dark:text-blue-100">
            当前真实开放能力：Grok 文生视频/首帧图生视频、OpenAI 视频模型、MiniMax-H3 文生视频/首帧图生视频。MiniMax-H3 的视频、音频混合参考能力暂未在本接口开放。
          </div>
          <div class="grid gap-3 md:grid-cols-2">
            <div class="rounded-lg border border-gray-200 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-900/60">
              <p class="font-medium text-gray-900 dark:text-white">服务地址</p>
              <pre class="mt-2 overflow-x-auto rounded-md bg-white p-3 text-xs dark:bg-dark-950"><code>{{ apiBaseURL }}/v1</code></pre>
            </div>
            <div class="rounded-lg border border-gray-200 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-900/60">
              <p class="font-medium text-gray-900 dark:text-white">请求头</p>
              <pre class="mt-2 overflow-x-auto rounded-md bg-white p-3 text-xs dark:bg-dark-950"><code>Authorization: Bearer YOUR_API_KEY
Content-Type: application/json</code></pre>
            </div>
          </div>
        </section>

        <section class="space-y-3">
          <h3 class="text-base font-semibold text-gray-900 dark:text-white">接口概览</h3>
          <div class="overflow-hidden rounded-lg border border-gray-200 dark:border-dark-700">
            <table class="min-w-full divide-y divide-gray-200 text-left text-xs dark:divide-dark-700">
              <thead class="bg-gray-50 text-gray-500 dark:bg-dark-900 dark:text-gray-400">
                <tr><th class="px-3 py-2">方法</th><th class="px-3 py-2">路径</th><th class="px-3 py-2">说明</th></tr>
              </thead>
              <tbody class="divide-y divide-gray-100 bg-white dark:divide-dark-800 dark:bg-dark-900">
                <tr><td class="px-3 py-2 font-mono">GET</td><td class="px-3 py-2 font-mono">/v1/models</td><td class="px-3 py-2">获取当前密钥可用模型，以返回结果为准。</td></tr>
                <tr><td class="px-3 py-2 font-mono">POST</td><td class="px-3 py-2 font-mono">/v1/videos/generations</td><td class="px-3 py-2">创建视频生成任务。</td></tr>
                <tr><td class="px-3 py-2 font-mono">GET</td><td class="px-3 py-2 font-mono">/v1/videos/tasks</td><td class="px-3 py-2">查询最近任务，支持 <code>limit</code>。</td></tr>
                <tr><td class="px-3 py-2 font-mono">GET</td><td class="px-3 py-2 font-mono">/v1/videos/{request_id}</td><td class="px-3 py-2">查询任务状态。</td></tr>
                <tr><td class="px-3 py-2 font-mono">GET</td><td class="px-3 py-2 font-mono">/v1/videos/{request_id}/content</td><td class="px-3 py-2">下载视频文件流。</td></tr>
              </tbody>
            </table>
          </div>
        </section>

        <section class="space-y-3">
          <h3 class="text-base font-semibold text-gray-900 dark:text-white">创建任务</h3>
          <p>通用写法适合 Grok、OpenAI 视频模型，也适合 MiniMax-H3 的文生视频。</p>
          <pre class="overflow-x-auto rounded-lg border border-gray-200 bg-gray-50 p-4 text-xs leading-6 dark:border-dark-700 dark:bg-dark-950"><code>curl '{{ apiBaseURL }}/v1/videos/generations' \
  -H 'Authorization: Bearer YOUR_API_KEY' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "MiniMax-H3",
    "prompt": "清晨的海边，镜头缓缓向前推进，阳光洒在海面",
    "duration": 8,
    "resolution": "768P",
    "aspect_ratio": "16:9"
  }'</code></pre>
          <p>MiniMax-H3 也支持 <code class="rounded bg-gray-100 px-1 py-0.5 text-xs dark:bg-dark-800">content</code> 数组写法；当前只接收一个文本项和可选的一张首帧图片。</p>
          <pre class="overflow-x-auto rounded-lg border border-gray-200 bg-gray-50 p-4 text-xs leading-6 dark:border-dark-700 dark:bg-dark-950"><code>curl '{{ apiBaseURL }}/v1/videos/generations' \
  -H 'Authorization: Bearer YOUR_API_KEY' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "MiniMax-H3",
    "content": [
      { "type": "text", "text": "让画面中的人物微笑并缓缓转身，镜头保持稳定" },
      { "type": "image_url", "role": "first_frame", "image_url": { "url": "https://assets.example.com/reference.jpg" } }
    ],
    "duration": 8,
    "resolution": "768P",
    "ratio": "16:9"
  }'</code></pre>
          <p class="text-xs text-gray-500 dark:text-gray-400">图片地址必须可公开读取。创作台内上传首帧会转成 data URI；第三方接口建议传 HTTPS 图片地址。</p>
        </section>

        <section class="space-y-3">
          <h3 class="text-base font-semibold text-gray-900 dark:text-white">查询与下载</h3>
          <pre class="overflow-x-auto rounded-lg border border-gray-200 bg-gray-50 p-4 text-xs leading-6 dark:border-dark-700 dark:bg-dark-950"><code>curl '{{ apiBaseURL }}/v1/videos/YOUR_REQUEST_ID' \
  -H 'Authorization: Bearer YOUR_API_KEY'</code></pre>
          <pre class="overflow-x-auto rounded-lg border border-gray-200 bg-gray-50 p-4 text-xs leading-6 dark:border-dark-700 dark:bg-dark-950"><code>curl '{{ apiBaseURL }}/v1/videos/YOUR_REQUEST_ID/content' \
  -H 'Authorization: Bearer YOUR_API_KEY' \
  --fail --output video.mp4</code></pre>
          <div class="rounded-lg border border-gray-200 bg-gray-50 p-3 text-xs dark:border-dark-700 dark:bg-dark-900/60">
            <p><strong>状态：</strong><code>pending</code>、<code>running</code> 表示处理中；<code>done</code> 或 <code>completed</code> 表示完成；<code>failed</code> 表示失败；<code>expired</code> 表示过期。</p>
            <p class="mt-1"><strong>保留：</strong>本站任务记录默认保留 {{ videoLimits.retentionDays }} 天，超过 {{ videoLimits.maxRecords }} 条会提前清理最早记录。视频文件有效期还受上游供应商限制，完成后请及时下载。</p>
          </div>
        </section>

        <section class="space-y-3">
          <h3 class="text-base font-semibold text-gray-900 dark:text-white">常见错误</h3>
          <div class="grid gap-2 text-xs sm:grid-cols-2">
            <p class="rounded-md bg-gray-50 p-2 dark:bg-dark-900"><code>401 / 403</code>：API Key、分组权限或额度不可用。</p>
            <p class="rounded-md bg-gray-50 p-2 dark:bg-dark-900"><code>402</code>：余额或套餐额度不足。</p>
            <p class="rounded-md bg-gray-50 p-2 dark:bg-dark-900"><code>404</code>：任务不存在、密钥不一致或记录已过期。</p>
            <p class="rounded-md bg-gray-50 p-2 dark:bg-dark-900"><code>409</code>：任务未就绪或重复请求冲突。</p>
            <p class="rounded-md bg-gray-50 p-2 dark:bg-dark-900"><code>429</code>：请求过快，请稍后重试。</p>
            <p class="rounded-md bg-gray-50 p-2 dark:bg-dark-900"><code>502 / 503</code>：上游暂不可用或文件失效。</p>
          </div>
        </section>
      </div>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" class="btn btn-secondary" @click="showVideoApiDocsModal = false">关闭</button>
          <button type="button" class="btn btn-primary" @click="copyVideoApiDocs">
            <Icon name="copy" size="sm" class="mr-2" />
            复制文档
          </button>
        </div>
      </template>
    </BaseDialog>

    <BaseDialog :show="showVideoPromptGuideModal" title="MiniMax-H3 · 提示词规范" width="extra-wide" :z-index="80" @close="showVideoPromptGuideModal = false">
      <div class="max-h-[72vh] space-y-6 overflow-y-auto pr-2 text-sm leading-6 text-gray-700 dark:text-gray-200">
        <section class="space-y-3">
          <h3 class="text-base font-semibold text-gray-900 dark:text-white">先写清楚画面，再写台词</h3>
          <p>把主体、动作、场景和镜头写在双引号外，把人物要说出的台词放进双引号内。描述简短、具体，一次突出一个主要动作。</p>
          <pre class="overflow-x-auto rounded-lg border border-gray-200 bg-gray-50 p-4 text-xs leading-6 dark:border-dark-700 dark:bg-dark-950"><code>一位设计师站在明亮的工作室里，拿起桌上的蓝色马克杯。
镜头从杯子特写缓缓拉到人物中景。
设计师微笑着说："为日常，留一点灵感。"</code></pre>
          <p class="text-xs text-gray-500 dark:text-gray-400">双引号有助于区分对白与画面描述，但生成结果仍可能出现台词、发音或字幕偏差。重要内容请在生成后检查。</p>
        </section>

        <section class="grid gap-4 lg:grid-cols-2">
          <div class="space-y-3 rounded-lg border border-gray-200 p-4 dark:border-dark-700">
            <h3 class="text-base font-semibold text-gray-900 dark:text-white">文生视频</h3>
            <p>用“主体 + 动作 + 环境 + 镜头”组织一句或几句描述。风格点到为止，不必堆叠相似形容词。</p>
            <pre class="overflow-x-auto rounded-md bg-gray-50 p-3 text-xs leading-6 dark:bg-dark-950"><code>清晨的玻璃温室，阳光穿过叶片，水滴顺着叶尖滑落。
镜头从水滴特写缓缓拉远，露出整排绿植，自然写实风格。</code></pre>
          </div>
          <div class="space-y-3 rounded-lg border border-gray-200 p-4 dark:border-dark-700">
            <h3 class="text-base font-semibold text-gray-900 dark:text-white">产品口播</h3>
            <p>引号中只放台词。画面、动作和声音要求写在引号外。台词长度要适合所选输出时长。</p>
            <pre class="overflow-x-auto rounded-md bg-gray-50 p-3 text-xs leading-6 dark:bg-dark-950"><code>一位店员站在书店窗边，手里展开一本素色笔记本。
镜头对准纸张细节，再移向人物。
店员说："把今天的灵感，留在这一页。"</code></pre>
          </div>
        </section>

        <section class="space-y-3">
          <h3 class="text-base font-semibold text-gray-900 dark:text-white">多人对白</h3>
          <p>每句台词前注明说话的人，先交代人物位置和交流顺序，减少过多分镜。当前创作台暂未开放音频参考上传，声音参考写法仅作为后续扩展规范。</p>
          <pre class="overflow-x-auto rounded-lg border border-gray-200 bg-gray-50 p-4 text-xs leading-6 dark:border-dark-700 dark:bg-dark-950"><code>咖啡店窗边，穿白衬衫的设计师坐在左侧，穿绿色外套的同事坐在右侧。
设计师把草图推到桌子中间，说："这个配色怎么样？"
同事看了一眼草图，说："很清爽，就用这个吧。"
镜头保持两人中景，背景是轻微的店内环境声。</code></pre>
        </section>

        <section class="space-y-3">
          <h3 class="text-base font-semibold text-gray-900 dark:text-white">首帧图生视频</h3>
          <p>首帧决定画面的起点，提示词补充接下来的动作与过渡。当前创作台已开放一张首帧图片；尾帧、参考视频和参考音频暂未开放。</p>
          <pre class="overflow-x-auto rounded-lg border border-gray-200 bg-gray-50 p-4 text-xs leading-6 dark:border-dark-700 dark:bg-dark-950"><code>镜头从起始画面缓慢向前推进，人物自然转身并抬手整理衣领，
光线平滑变化，最终停留在明亮自然的中景构图。</code></pre>
        </section>

        <section class="space-y-3">
          <h3 class="text-base font-semibold text-gray-900 dark:text-white">只保留环境音</h3>
          <pre class="overflow-x-auto rounded-lg border border-gray-200 bg-gray-50 p-4 text-xs leading-6 dark:border-dark-700 dark:bg-dark-950"><code>雨滴落在窗玻璃上，室内桌面放着一杯冒着热气的茶。
镜头固定，远处灯光轻轻闪动。
无对白、无旁白，只保留轻微雨声和室内环境音。</code></pre>
        </section>

        <section class="space-y-3">
          <h3 class="text-base font-semibold text-gray-900 dark:text-white">精简提示词</h3>
          <div class="overflow-hidden rounded-lg border border-gray-200 dark:border-dark-700">
            <table class="min-w-full divide-y divide-gray-200 text-left text-xs dark:divide-dark-700">
              <thead class="bg-gray-50 text-gray-500 dark:bg-dark-900 dark:text-gray-400">
                <tr><th class="px-3 py-2">容易干扰生成的写法</th><th class="px-3 py-2">更合适的写法</th></tr>
              </thead>
              <tbody class="divide-y divide-gray-100 bg-white dark:divide-dark-800 dark:bg-dark-900">
                <tr><td class="px-3 py-2">引号里混入全局指令、高清、运镜要求</td><td class="px-3 py-2">引号里只留人物台词</td></tr>
                <tr><td class="px-3 py-2">反复堆叠“4K、2K、最高画质”</td><td class="px-3 py-2">在参数区选择分辨率，提示词描述场景质感</td></tr>
                <tr><td class="px-3 py-2">大段“禁止、必须、严格遵守”的指令块</td><td class="px-3 py-2">直接描述希望出现的动作、画面与声音</td></tr>
                <tr><td class="px-3 py-2">长段口播塞进几秒的视频</td><td class="px-3 py-2">缩短台词，或适当增加输出时长</td></tr>
                <tr><td class="px-3 py-2">要求台词百分之百准确</td><td class="px-3 py-2">生成后检查，有严格要求时后期配音</td></tr>
                <tr><td class="px-3 py-2">自定义 VoiceID 或未绑定的素材标记</td><td class="px-3 py-2">当前版本不要写未上传的素材标记</td></tr>
              </tbody>
            </table>
          </div>
          <p class="text-xs text-gray-500 dark:text-gray-400">需要准确字幕时，建议在成片后添加字幕。先用简洁提示词确认构图和动作，再逐步调整细节。</p>
        </section>
      </div>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" class="btn btn-secondary" @click="showVideoPromptGuideModal = false">关闭</button>
        </div>
      </template>
    </BaseDialog>

    <Teleport to="body">
      <div
        v-if="openMoreJobId"
        class="fixed z-[9999] w-44 overflow-hidden rounded-xl bg-white py-1 text-sm shadow-lg ring-1 ring-black/5 dark:bg-dark-800 dark:ring-white/10"
        :style="moreMenuStyle"
        @click.stop
      >
        <template v-for="job in batchJobs" :key="job.id">
          <template v-if="job.id === openMoreJobId">
            <button
              v-if="canRetry(job)"
              type="button"
              class="flex w-full items-center gap-2 px-3 py-2 text-left text-gray-700 transition-colors hover:bg-amber-50 hover:text-amber-700 disabled:opacity-60 dark:text-gray-200 dark:hover:bg-amber-900/20 dark:hover:text-amber-300"
              :disabled="retryingBatchId === job.id"
              @click="retryFailedJob(job)"
            >
              <Icon name="refresh" size="sm" :class="retryingBatchId === job.id ? 'animate-spin' : ''" />
              {{ t('batchImage.actions.retryFailedItems') }}
            </button>
            <button
              v-if="canDeleteRecord(job)"
              type="button"
              class="flex w-full items-center gap-2 px-3 py-2 text-left text-red-600 transition-colors hover:bg-red-50 disabled:opacity-60 dark:text-red-400 dark:hover:bg-red-900/20"
              :disabled="deletingBatchId === job.id"
              @click="deleteJob(job)"
            >
              <Icon :name="deletingBatchId === job.id ? 'refresh' : 'trash'" size="sm" :class="deletingBatchId === job.id ? 'animate-spin' : ''" />
              {{ t('batchImage.actions.deleteRecords') }}
            </button>
          </template>
        </template>
      </div>
    </Teleport>

    <Teleport to="body">
      <div
        v-if="promptPopover.visible"
        class="batch-prompt-popover fixed z-[9999] rounded-lg border border-gray-200 bg-white p-3 text-sm text-gray-800 shadow-xl ring-1 ring-black/5 dark:border-dark-700 dark:bg-dark-900 dark:text-gray-100 dark:ring-white/10"
        :style="promptPopover.style"
        @mouseenter="cancelPromptPopoverClose"
        @mouseleave="schedulePromptPopoverClose"
      >
        <div class="mb-2 flex items-center justify-between gap-3">
          <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('batchImage.promptPopover.title') }}</span>
          <button
            type="button"
            class="rounded-md px-2 py-1 text-xs font-medium text-primary-600 transition-colors hover:bg-primary-50 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/30 dark:text-primary-300 dark:hover:bg-primary-900/20"
            @click="copyPromptPopover"
          >
            {{ t('common.copy') }}
          </button>
        </div>
        <p class="max-h-48 overflow-y-auto whitespace-pre-wrap break-words leading-6 selection:bg-primary-100 selection:text-primary-900 dark:selection:bg-primary-900/60 dark:selection:text-primary-100">
          {{ promptPopover.text }}
        </p>
      </div>
    </Teleport>

    <BaseDialog :show="!!currentJob" :title="t('batchImage.detail.title')" width="extra-wide" @close="closeDetail">
      <div v-if="currentJob" class="space-y-4">
        <div class="rounded-lg border border-gray-200 bg-gray-50/70 px-4 py-3 dark:border-dark-700 dark:bg-dark-900/40">
          <div class="grid gap-x-6 gap-y-3 sm:grid-cols-2 lg:grid-cols-4">
            <div class="min-w-0 text-center">
              <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('common.status') }}</p>
              <div class="mt-1 flex justify-center">
                <span :class="statusBadgeClass(currentDisplayJob || currentJob)" class="badge whitespace-nowrap">
                  {{ statusLabel(currentDisplayJob || currentJob) }}
                </span>
              </div>
            </div>
            <div class="min-w-0 text-center">
              <p class="text-xs text-gray-500 dark:text-gray-400">{{ hasChildJobs(currentJob.id) ? t('batchImage.detail.aggregatedResult') : t('batchImage.detail.result') }}</p>
              <p class="mt-1 flex items-center justify-center gap-2 font-medium tabular-nums">
              <span class="text-emerald-600 dark:text-emerald-300">{{ (currentDisplayJob || currentJob).success_count }}</span>
              <span class="text-gray-300 dark:text-dark-500">/</span>
              <span :class="(currentDisplayJob || currentJob).fail_count > 0 ? 'text-red-600 dark:text-red-300' : 'text-gray-400 dark:text-gray-500'">{{ (currentDisplayJob || currentJob).fail_count }}</span>
            </p>
            </div>
            <div class="min-w-0 text-center">
              <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('batchImage.detail.cost') }}</p>
              <p class="mt-1 truncate font-medium text-gray-900 dark:text-white">{{ costLabel(currentDisplayJob || currentJob) }}</p>
            </div>
            <div class="min-w-0 text-center">
              <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('batchImage.detail.downloadStatus') }}</p>
              <p class="mt-1 truncate font-medium text-gray-900 dark:text-white">
              {{ currentJob.downloaded_at ? formatDate(currentJob.downloaded_at) : t('batchImage.list.notDownloaded') }}
            </p>
            </div>
          </div>
        </div>

        <div class="flex flex-wrap items-center justify-between gap-3">
          <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('batchImage.detail.items') }}</h3>
          <button type="button" class="btn btn-secondary btn-sm" :disabled="refreshing || loadingItems" @click="refreshDetail">
            <Icon name="refresh" size="sm" class="mr-1.5" :class="refreshing || loadingItems ? 'animate-spin' : ''" />
            {{ t('common.refresh') }}
          </button>
        </div>

        <div v-if="items.length" class="overflow-x-auto rounded-lg border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-900">
          <table class="w-full min-w-[860px] table-fixed divide-y divide-gray-200 text-sm dark:divide-dark-700">
            <colgroup>
              <col class="w-[18%]" />
              <col class="w-[34%]" />
              <col class="w-[12%]" />
              <col class="w-[10%]" />
              <col class="w-[26%]" />
            </colgroup>
            <thead class="bg-gray-50 dark:bg-dark-800/80">
              <tr>
                <th class="px-3 py-3 text-center text-sm font-medium text-gray-500 dark:text-gray-400">Custom ID</th>
                <th class="px-3 py-3 text-left text-sm font-medium text-gray-500 dark:text-gray-400">Prompt</th>
                <th class="px-3 py-3 text-center text-sm font-medium text-gray-500 dark:text-gray-400">{{ t('common.status') }}</th>
                <th class="px-3 py-3 text-center text-sm font-medium text-gray-500 dark:text-gray-400">{{ t('batchImage.detail.preview') }}</th>
                <th class="px-3 py-3 text-center text-sm font-medium text-gray-500 dark:text-gray-400">{{ t('batchImage.detail.result') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr
                v-for="item in items"
                :key="itemPreviewKey(item)"
                class="align-middle"
                :class="detailItemRowClass(item)"
              >
                <td class="px-3 py-2.5 text-center">
                  <span
                    class="block min-w-0 truncate font-mono text-sm"
                    :class="isRecoveredOriginalFailure(item) ? 'text-gray-400 dark:text-gray-500' : 'text-gray-900 dark:text-white'"
                    :title="item.custom_id"
                  >
                    {{ item.custom_id }}
                  </span>
                </td>
                <td class="px-3 py-2.5 text-left" :class="isRecoveredOriginalFailure(item) ? 'text-gray-400 dark:text-gray-500' : 'text-gray-700 dark:text-gray-300'">
                  <div
                    class="batch-prompt-trigger cursor-default truncate rounded px-1 text-sm leading-6 focus:outline-none"
                    tabindex="0"
                    @pointerenter="schedulePromptPopoverOpen($event, item.prompt_preview || '-')"
                    @pointerleave="schedulePromptPopoverClose"
                    @mouseenter="schedulePromptPopoverOpen($event, item.prompt_preview || '-')"
                    @mouseleave="schedulePromptPopoverClose"
                    @click="showPromptPopover($event, item.prompt_preview || '-')"
                    @focus="showPromptPopover($event, item.prompt_preview || '-')"
                    @focusin="showPromptPopover($event, item.prompt_preview || '-')"
                    @blur="schedulePromptPopoverClose"
                  >
                    {{ item.prompt_preview || '-' }}
                  </div>
                </td>
                <td class="px-3 py-2.5 text-center">
                  <span :class="itemDisplayStatusBadgeClass(item)" class="badge max-w-full truncate whitespace-nowrap" :title="itemDisplayStatusLabel(item)">
                    {{ itemDisplayStatusLabel(item) }}
                  </span>
                </td>
                <td class="px-3 py-2.5 text-center">
                  <div class="mx-auto h-12 w-12 overflow-hidden rounded-md border border-gray-200 bg-gray-50 dark:border-dark-700 dark:bg-dark-800">
                    <button
                      v-if="itemPreviewUrls[itemPreviewKey(item)] && !previewErrorIds.has(itemPreviewKey(item))"
                      type="button"
                      class="block h-full w-full overflow-hidden"
                      :title="t('batchImage.detail.previewZoom', { id: item.custom_id })"
                      @click="openImagePreview(item)"
                    >
                      <img
                        :src="itemPreviewUrls[itemPreviewKey(item)]"
                        class="h-full w-full object-cover"
                        alt=""
                        @error="handlePreviewError(itemPreviewKey(item))"
                      />
                    </button>
                    <button
                      v-else-if="canLoadItemPreview(item)"
                      type="button"
                      class="flex h-full w-full items-center justify-center text-gray-500 transition-colors hover:bg-gray-100 hover:text-primary-600 disabled:cursor-wait disabled:opacity-70 dark:text-gray-400 dark:hover:bg-dark-700"
                      :disabled="previewLoadingIds.has(itemPreviewKey(item))"
                      :title="previewErrorIds.has(itemPreviewKey(item)) ? t('batchImage.detail.previewReload') : t('batchImage.detail.previewLoad')"
                      @click="loadItemPreview(item)"
                    >
                      <Icon :name="previewLoadingIds.has(itemPreviewKey(item)) ? 'refresh' : 'eye'" size="sm" :class="previewLoadingIds.has(itemPreviewKey(item)) ? 'animate-spin' : ''" />
                    </button>
                    <div v-else class="flex h-full w-full items-center justify-center text-gray-400" :title="item.image_count > 0 ? t('batchImage.detail.previewUnavailable') : t('batchImage.detail.noImage')">
                      <Icon name="document" size="sm" />
                    </div>
                  </div>
                </td>
                <td class="px-3 py-2.5 text-center">
                  <span
                    class="inline-flex max-w-full items-center justify-center truncate rounded-md px-2.5 py-1 text-xs font-medium leading-5 ring-1 ring-inset"
                    :class="itemResultClass(item)"
                    :title="itemResultLabel(item)"
                  >
                    {{ itemResultLabel(item) }}
                  </span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-else class="rounded-lg border border-dashed border-gray-200 py-10 text-center dark:border-dark-700">
          <Icon name="refresh" size="lg" class="mx-auto mb-3 text-gray-400" :class="loadingItems ? 'animate-spin' : ''" />
          <p class="text-sm font-medium text-gray-700 dark:text-gray-200">
            {{ loadingItems ? t('batchImage.detail.loadingItems') : t('batchImage.detail.noItems') }}
          </p>
          <p v-if="!loadingItems" class="mt-1 text-sm text-gray-500 dark:text-gray-400">
            {{ t('batchImage.detail.noItemsHint') }}
          </p>
        </div>
      </div>

      <template #footer>
        <div class="flex justify-end gap-3">
	          <button type="button" class="btn btn-secondary" :disabled="!currentJob || !canCancel(currentJob) || cancelling" @click="cancelSelected">
	            <Icon v-if="cancelling" name="refresh" size="sm" class="mr-2 animate-spin" />
	            {{ t('batchImage.actions.cancelJob') }}
	          </button>
	          <button
	            v-if="currentJob && currentDisplayJob && canRetry(currentDisplayJob)"
	            type="button"
	            class="btn btn-secondary inline-flex min-w-[116px] items-center justify-center"
	            :disabled="retryingBatchId === currentJob.id"
	            @click="retrySelected"
	          >
	            <Icon name="refresh" size="sm" class="mr-2" :class="currentJob && retryingBatchId === currentJob.id ? 'animate-spin' : ''" />
	            {{ t('batchImage.actions.retryFailedItems') }}
	          </button>
	          <button
            type="button"
            class="btn btn-primary inline-flex min-w-[112px] items-center justify-center"
            :disabled="!currentJob || !canDownload(currentJob) || downloading"
            @click="downloadSelected"
          >
            <Icon
              :name="currentJob && isDownloadingJob(currentJob.id) ? 'refresh' : 'download'"
              size="sm"
              class="mr-2"
              :class="currentJob && isDownloadingJob(currentJob.id) ? 'animate-spin' : ''"
            />
            {{ t('batchImage.actions.downloadZip') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <BaseDialog
      :show="!!previewImageItem"
      :title="previewImageItem?.custom_id || t('batchImage.imagePreview.title')"
      width="full"
      panel-class="image-preview-dialog"
      :z-index="60"
      @close="closeImagePreview"
    >
      <div v-if="previewImageItem" class="grid min-h-[76vh] gap-3 lg:min-h-[82vh] lg:grid-cols-[auto_minmax(0,1fr)_300px] xl:grid-cols-[auto_minmax(0,1fr)_320px]">
        <div
          v-if="previewImageCandidates.length > 1"
          class="order-2 flex max-h-[18vh] min-h-0 gap-2 overflow-x-auto rounded-lg border border-gray-200 bg-white p-2 dark:border-dark-700 dark:bg-dark-900 lg:order-1 lg:max-h-[82vh] lg:w-[76px] lg:flex-col lg:overflow-y-auto lg:overflow-x-hidden"
        >
          <button
            v-for="item in previewImageCandidates"
            :key="itemPreviewKey(item)"
            type="button"
            class="relative h-14 w-14 flex-shrink-0 overflow-hidden rounded-md border-2 bg-gray-100 transition dark:bg-dark-800"
            :class="previewImageItem === item ? 'border-primary-500 ring-2 ring-primary-500/20' : 'border-transparent hover:border-gray-300 dark:hover:border-dark-500'"
            :title="item.custom_id"
            @click="selectPreviewImage(item)"
          >
            <img
              v-if="itemPreviewUrls[itemPreviewKey(item)] && !previewErrorIds.has(itemPreviewKey(item))"
              :src="itemPreviewUrls[itemPreviewKey(item)]"
              class="h-full w-full object-cover"
              alt=""
            />
            <span v-else class="flex h-full w-full items-center justify-center text-gray-400">
              <Icon :name="previewLoadingIds.has(itemPreviewKey(item)) ? 'refresh' : 'eye'" size="sm" :class="previewLoadingIds.has(itemPreviewKey(item)) ? 'animate-spin' : ''" />
            </span>
          </button>
        </div>
        <div class="relative order-1 flex min-h-[62vh] items-center justify-center overflow-hidden rounded-lg bg-gray-950 p-2 sm:min-h-[68vh] lg:order-2 lg:min-h-[82vh]">
          <img
            v-if="previewImageDisplayUrl"
            :src="previewImageDisplayUrl"
            class="h-full max-h-[82vh] w-full rounded-md object-contain"
            :alt="previewImageItem.custom_id || ''"
            @error="handleImagePreviewDisplayError"
          />
          <div v-else class="flex flex-col items-center gap-3 text-sm text-white/70">
            <Icon name="refresh" size="lg" class="animate-spin" />
            {{ t('batchImage.imagePreview.loading') }}
          </div>
          <div v-if="previewImageFullLoading" class="absolute left-3 top-3 inline-flex items-center rounded-md bg-black/60 px-2.5 py-1.5 text-xs font-medium text-white">
            <Icon name="refresh" size="xs" class="mr-1.5 animate-spin" />
            {{ t('batchImage.imagePreview.loadingOriginal') }}
          </div>
          <div v-if="previewImageFullError" class="absolute inset-x-3 bottom-3 rounded-md bg-black/70 px-3 py-2 text-sm text-white">
            {{ previewImageFullError }}
          </div>
        </div>
        <div class="order-3 flex min-h-0 flex-col gap-3 overflow-y-auto lg:max-h-[82vh]">
          <div class="rounded-lg border border-gray-200 bg-white p-3 dark:border-dark-700 dark:bg-dark-900">
            <div class="mb-3 flex items-center justify-between gap-2">
              <span :class="itemDisplayStatusBadgeClass(previewImageItem)" class="badge whitespace-nowrap">
                {{ itemDisplayStatusLabel(previewImageItem) }}
              </span>
              <span class="text-xs text-gray-500 dark:text-gray-400">{{ previewImageIndexText }}</span>
            </div>
            <div class="space-y-2">
              <div class="flex items-center justify-between gap-2">
                <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('batchImage.imagePreview.prompt') }}</h3>
                <button type="button" class="btn-ghost btn-icon" :title="t('batchImage.imagePreview.copyPrompt')" @click="copyPreviewImagePrompt">
                  <Icon name="copy" size="sm" />
                </button>
              </div>
              <p class="max-h-36 overflow-y-auto rounded-lg bg-gray-50 p-3 text-sm leading-6 text-gray-700 dark:bg-dark-800 dark:text-gray-300">
                {{ previewImageItem.prompt_preview || '-' }}
              </p>
            </div>
          </div>

          <div class="rounded-lg border border-gray-200 bg-white p-3 dark:border-dark-700 dark:bg-dark-900">
            <h3 class="mb-3 text-sm font-semibold text-gray-900 dark:text-white">{{ t('batchImage.imagePreview.details') }}</h3>
            <dl class="grid grid-cols-2 gap-x-4 gap-y-3 text-sm">
              <dt class="text-gray-500 dark:text-gray-400">Custom ID</dt>
              <dd class="min-w-0 truncate text-right font-mono text-gray-900 dark:text-white" :title="previewImageItem.custom_id">{{ previewImageItem.custom_id }}</dd>
              <dt class="text-gray-500 dark:text-gray-400">{{ t('batchImage.imagePreview.sourceTask') }}</dt>
              <dd class="min-w-0 truncate text-right text-gray-900 dark:text-white" :title="previewImageItem.source_task_name">{{ previewImageItem.source_task_name || '-' }}</dd>
              <dt class="text-gray-500 dark:text-gray-400">{{ t('batchImage.imagePreview.model') }}</dt>
              <dd class="min-w-0 truncate text-right text-gray-900 dark:text-white" :title="previewImageJobModel">{{ previewImageJobModel }}</dd>
              <dt class="text-gray-500 dark:text-gray-400">{{ t('batchImage.imagePreview.format') }}</dt>
              <dd class="text-right text-gray-900 dark:text-white">{{ previewImageFormatText }}</dd>
              <dt class="text-gray-500 dark:text-gray-400">{{ t('batchImage.imagePreview.fileSize') }}</dt>
              <dd class="text-right text-gray-900 dark:text-white">{{ previewImageFileSizeText }}</dd>
              <dt class="text-gray-500 dark:text-gray-400">{{ t('batchImage.imagePreview.cost') }}</dt>
              <dd class="text-right text-gray-900 dark:text-white">{{ previewImageJobCost }}</dd>
              <dt class="text-gray-500 dark:text-gray-400">{{ t('batchImage.imagePreview.createdAt') }}</dt>
              <dd class="text-right text-gray-900 dark:text-white">{{ previewImageCreatedAt }}</dd>
            </dl>
          </div>

          <div class="mt-auto grid gap-2 border-t border-gray-200 pt-3 dark:border-dark-700">
            <button type="button" class="btn btn-primary w-full justify-center" :disabled="previewImageDownloading" @click="downloadPreviewImageOriginal">
              <Icon :name="previewImageDownloading ? 'refresh' : 'download'" size="sm" class="mr-2" :class="previewImageDownloading ? 'animate-spin' : ''" />
              {{ t('batchImage.imagePreview.downloadOriginal') }}
            </button>
            <div class="grid grid-cols-2 gap-2">
              <button type="button" class="btn btn-secondary justify-center" :disabled="previewImageFullLoading" @click="reloadPreviewImageOriginal">
                <Icon name="refresh" size="sm" class="mr-2" :class="previewImageFullLoading ? 'animate-spin' : ''" />
                {{ t('common.refresh') }}
              </button>
              <button type="button" class="btn btn-secondary justify-center" @click="downloadPreviewImageJob">
                <Icon name="download" size="sm" class="mr-2" />
                {{ t('batchImage.actions.downloadZip') }}
              </button>
            </div>
          </div>
        </div>
      </div>
    </BaseDialog>

    <BaseDialog :show="showCreateModal" :title="t('batchImage.create.title')" width="wide" @close="closeCreateModal">
      <form class="space-y-5" @submit.prevent="submitJob">
        <div class="grid gap-4 md:grid-cols-2">
          <div class="md:col-span-2">
            <label class="input-label">{{ t('batchImage.create.taskName') }}</label>
            <input
              v-model="form.taskName"
              type="text"
              maxlength="255"
              class="input"
              :placeholder="t('batchImage.create.taskNamePlaceholder')"
            />
          </div>

          <div class="md:col-span-2">
            <label class="input-label">API Key</label>
            <select v-model.number="form.apiKeyId" class="input" :disabled="loadingKeys">
              <option :value="0">{{ loadingKeys ? t('batchImage.create.loadingKeys') : t('batchImage.create.selectKeyPlaceholder') }}</option>
              <option v-for="key in geminiApiKeys" :key="key.id" :value="key.id">
                {{ key.name }} · {{ key.group?.name || 'Gemini' }}
              </option>
            </select>
            <p v-if="!loadingKeys && geminiApiKeys.length === 0" class="input-hint text-amber-600 dark:text-amber-400">
              {{ t('batchImage.create.noKeysHint') }}
            </p>
          </div>

          <div>
            <label class="input-label">{{ t('batchImage.create.model') }}</label>
            <select v-model="form.model" class="input" :disabled="loadingModels || availableBatchImageModels.length === 0">
              <option v-if="loadingModels" value="">{{ batchImageText('loadingModels') }}</option>
              <option v-else-if="availableBatchImageModels.length === 0" value="">{{ batchImageText('noModels') }}</option>
              <option v-for="model in availableBatchImageModels" :key="model.value" :value="model.value">
                {{ model.label }}
              </option>
            </select>
            <p v-if="modelLoadError" class="input-hint text-amber-600 dark:text-amber-400">
              {{ modelLoadError }}
            </p>
            <p v-else-if="selectedApiKey && !loadingModels && availableBatchImageModels.length === 0" class="input-hint text-amber-600 dark:text-amber-400">
              {{ batchImageText('noModelsHint') }}
            </p>
          </div>

          <div>
            <label class="input-label">{{ t('batchImage.create.imageSize') }}</label>
            <div class="input flex items-center bg-gray-50 text-gray-600 dark:bg-dark-900 dark:text-gray-300">
              1K
            </div>
            <p class="input-hint">{{ t('batchImage.create.imageSizeHint') }}</p>
          </div>

          <div>
            <label class="input-label">{{ t('batchImage.create.outputFormat') }}</label>
            <select v-model="form.responseMimeType" class="input">
              <option value="image/png">PNG</option>
              <option value="image/jpeg">JPEG</option>
              <option value="image/webp">WebP</option>
            </select>
          </div>

          <div>
            <label class="input-label">{{ t('batchImage.create.estimatedOutput') }}</label>
            <div class="input flex items-center bg-gray-50 text-gray-600 dark:bg-dark-900 dark:text-gray-300">
              {{ t('batchImage.create.estimatedOutputValue', { images: estimatedOutputCount, prompts: promptRows.length }) }}
            </div>
          </div>
        </div>

        <div class="space-y-3">
          <div class="flex items-center justify-between gap-3">
            <label class="input-label mb-0">Prompt</label>
            <span class="text-xs text-gray-500 dark:text-gray-400">{{ t('batchImage.create.promptAdded', { count: promptRows.length }) }}</span>
          </div>
          <div class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
            <textarea
              v-model="promptDraft"
              rows="3"
              class="h-[76px] w-full resize-y rounded-md border border-gray-300 px-3 py-2 text-sm leading-5 outline-none focus:border-primary-500 focus:ring-2 focus:ring-primary-100 dark:border-dark-600 dark:bg-dark-900 dark:text-gray-100 dark:focus:border-primary-500 dark:focus:ring-primary-900/40"
              :placeholder="t('batchImage.create.promptPlaceholder')"
            />
            <div class="mt-2 grid gap-2 md:grid-cols-[minmax(0,1fr)_112px_132px_112px] md:items-center">
              <input
                v-model="customIdDraft"
                type="text"
                maxlength="255"
                class="input h-9 text-sm"
                :placeholder="t('batchImage.create.customIdPlaceholder')"
              />
              <select
                v-model.number="outputCountDraft"
                class="batch-output-count-select input h-9 text-sm"
                :title="t('batchImage.create.outputCountPerPrompt')"
                :aria-label="t('batchImage.create.outputCountPerPrompt')"
              >
                <option v-for="count in outputCountOptions" :key="count" :value="count">
                  {{ t('batchImage.create.outputCountOption', { n: count }, count) }}
                </option>
              </select>
              <label
                class="btn btn-secondary h-9 cursor-pointer justify-center text-sm"
                :class="referenceImageDrafts.length >= selectedModelReferenceLimit ? 'pointer-events-none opacity-60' : ''"
              >
                <Icon name="upload" size="sm" class="mr-1.5" />
                {{ t('batchImage.create.referenceImage') }}
                <input
                  type="file"
                  accept="image/png,image/jpeg,image/webp"
                  multiple
                  class="hidden"
                  :disabled="referenceImageDrafts.length >= selectedModelReferenceLimit"
                  @change="handleReferenceImageFiles"
                />
              </label>
              <button type="button" class="btn btn-secondary h-9 justify-center whitespace-nowrap px-4 text-sm" :disabled="!promptDraft.trim()" @click="addPromptRow">
                <Icon name="plus" size="sm" class="mr-1.5" />
                {{ t('common.add') }}
              </button>
            </div>
            <div v-if="referenceImageDrafts.length" class="mt-3 flex flex-wrap gap-2">
              <span
                v-for="(ref, refIndex) in referenceImageDrafts"
                :key="`${ref.name}-${refIndex}`"
                class="inline-flex max-w-full items-center gap-1 rounded-md border border-gray-200 bg-gray-50 px-2 py-1 text-xs text-gray-700 dark:border-dark-700 dark:bg-dark-900 dark:text-gray-200"
              >
                <span class="max-w-[180px] truncate">{{ ref.name }}</span>
                <button type="button" class="text-gray-400 hover:text-red-600" :title="t('batchImage.create.removeReferenceImage')" @click="removeReferenceImageDraft(refIndex)">
                  <Icon name="x" size="xs" />
                </button>
              </span>
            </div>
            <p class="mt-2 text-xs text-gray-500 dark:text-gray-400">
              {{ t('batchImage.create.limitsHint', { maxPerItem: BATCH_IMAGE_MAX_OUTPUTS_PER_ITEM, maxPerJob: BATCH_IMAGE_MAX_OUTPUTS_PER_JOB, refLimit: selectedModelReferenceLimit }) }}
            </p>
          </div>
          <div v-if="promptRows.length" class="overflow-hidden rounded-lg border border-gray-200 dark:border-dark-700">
            <div
              v-for="(row, index) in promptRows"
              :key="row.localId"
              class="flex items-center gap-3 border-b border-gray-100 px-3 py-2 last:border-b-0 dark:border-dark-700"
            >
              <span class="w-20 flex-shrink-0 font-mono text-xs text-gray-500 dark:text-gray-400">{{ row.custom_id }}</span>
              <p class="min-w-0 flex-1 truncate text-sm text-gray-800 dark:text-gray-100">{{ row.prompt }}</p>
              <span v-if="row.output_count > 1" class="flex-shrink-0 text-xs text-gray-500 dark:text-gray-400">
                x{{ row.output_count }}
              </span>
              <span v-if="row.reference_images.length" class="flex-shrink-0 text-xs text-gray-500 dark:text-gray-400">
                {{ t('batchImage.create.referenceCount', { n: row.reference_images.length }, row.reference_images.length) }}
              </span>
              <button type="button" class="btn-ghost btn-icon flex-shrink-0 text-red-600 hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-900/20" :title="t('common.delete')" @click="removePromptRow(index)">
                <Icon name="trash" size="sm" />
              </button>
            </div>
          </div>
          <div v-else class="rounded-lg border border-dashed border-gray-200 px-3 py-6 text-center text-sm text-gray-500 dark:border-dark-700 dark:text-gray-400">
            {{ t('batchImage.create.noPrompts') }}
          </div>
        </div>

	        <div class="rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm leading-6 text-amber-900 dark:border-amber-800 dark:bg-amber-950/30 dark:text-amber-100">
	          {{ t('batchImage.create.cancelNotice') }}
	        </div>
	        <div v-if="submitting" class="rounded-lg border border-sky-200 bg-sky-50 p-3 text-sm leading-6 text-sky-800 dark:border-sky-800 dark:bg-sky-950/30 dark:text-sky-100">
	          {{ t('batchImage.create.submittingNotice') }}
	        </div>
	      </form>

      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" class="btn btn-secondary" :disabled="submitting" @click="closeCreateModal">{{ t('common.cancel') }}</button>
          <button type="button" class="btn btn-primary inline-flex min-w-[120px] justify-center" :disabled="submitting || (loadingModels && !form.model) || (parsedItems.length === 0 && !promptDraft.trim()) || !selectedApiKey || !form.model" @click="submitJob">
            <Icon v-if="submitting" name="refresh" size="sm" class="mr-2 animate-spin" />
            {{ submitting ? t('common.submitting') : t('batchImage.actions.submitJob') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <BaseDialog :show="showGuideModal" :title="t('batchImage.guide.title')" width="wide" @close="showGuideModal = false">
	      <div class="space-y-5">
	        <section class="space-y-3">
	          <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('batchImage.guide.uiTitle') }}</h3>
	          <div class="rounded-lg border border-gray-200 bg-gray-50 p-3 text-sm leading-6 text-gray-700 dark:border-dark-700 dark:bg-dark-900/50 dark:text-gray-200">
	            <p>{{ t('batchImage.guide.step1') }}</p>
	            <p>{{ t('batchImage.guide.step2') }}</p>
	            <p>{{ t('batchImage.guide.step3') }}</p>
	            <p>{{ t('batchImage.guide.step4') }}</p>
	          </div>
	        </section>
	        <section class="space-y-3">
	          <div class="flex flex-wrap items-center justify-between gap-3">
	            <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('batchImage.guide.skillTitle') }}</h3>
	            <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('batchImage.guide.skillDesc') }}</p>
	          </div>
	        <textarea
	          :value="agentInstruction"
	          readonly
	          class="min-h-[420px] w-full resize-y rounded-md border border-gray-200 bg-gray-50 p-4 font-mono text-sm leading-6 text-gray-800 outline-none focus:border-primary-400 focus:ring-2 focus:ring-primary-100 dark:border-dark-600 dark:bg-dark-900 dark:text-gray-100 dark:focus:border-primary-500 dark:focus:ring-primary-900/40"
	        />
	        </section>
	      </div>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" class="btn btn-secondary" @click="showGuideModal = false">{{ t('common.close') }}</button>
          <button type="button" class="btn btn-primary" @click="copyInstruction">
            <Icon name="copy" size="sm" class="mr-2" />
            {{ t('batchImage.actions.copyInstruction') }}
          </button>
        </div>
      </template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select, { type SelectOption } from '@/components/common/Select.vue'
import SearchInput from '@/components/common/SearchInput.vue'
import Icon from '@/components/icons/Icon.vue'
import { useClipboard } from '@/composables/useClipboard'
import { getPersistedPageSize, setPersistedPageSize } from '@/composables/usePersistedPageSize'
import { useAppStore } from '@/stores/app'
import { keysAPI } from '@/api'
import {
  cancelBatchImageJob,
  deleteBatchImageJobRecord,
  downloadBatchImageZip,
  getBatchImageItemContent,
  getBatchImageJob,
  listBatchImageJobs,
  listBatchImageItems,
  listBatchImageModels,
  saveBlob,
  submitBatchImageJob,
  type BatchImageItem,
  type BatchImageJob,
  type BatchImageJobsListOptions,
  type BatchImageReferenceImage,
  type BatchImageStatus,
  type BatchImageSubmitItem,
} from '@/api/batchImage'
import {
  createCreativeVideo,
  deleteCreativeVideoTask,
  downloadCreativeVideo,
  getCreativeVideoStatus,
  listCreativeVideoTasks,
  type CreativeVideoTask,
} from '@/api/creativeVideo'
import type { ApiKey } from '@/types'
import type { Column } from '@/components/common/types'

type BatchImageJobRow = Pick<BatchImageJob, 'id' | 'task_name' | 'parent_batch_id' | 'status' | 'model' | 'provider' | 'item_count' | 'success_count' | 'fail_count' | 'estimated_cost' | 'hold_amount' | 'actual_cost' | 'created_at' | 'downloaded_at'> & {
  api_key_id: number
  api_key_name: string
  child_count: number
  is_child?: boolean
}

type BatchImageDetailItem = BatchImageItem & {
  batch_id: string
  source_task_name: string
}

type PromptRow = {
  localId: string
  custom_id: string
  prompt: string
  output_count: number
  reference_images: BatchImageReferenceImage[]
}

type ReferenceImageDraft = BatchImageReferenceImage & {
  name: string
  size: number
}

type VideoFrameDraft = {
  name: string
  mimeType: string
  size: number
  data: string
}

type ImageCreativeTool = 'text' | 'edit'
type TemplateDrawerState = 'collapsed' | 'rail' | 'expanded'

type CreativeTemplate = {
  id: string
  title: string
  description: string
  mode: ImageCreativeTool
  category: string
  prompt: string
  aspectRatio: string
  size: string
  count: number
  previewClass: string
}

type ModelChoice = {
  id: string
  apiKeyId: number
  label: string
  maskedKey: string
  platformLabel: string
  selected: boolean
  disabled: boolean
  reason: string
  mode: 'image' | 'video'
}

type PreviewCacheRecord = {
  key: string
  blob: Blob
  size: number
  createdAt: number
  lastAccessedAt: number
}

type PreviewImageSource = ImageBitmap | HTMLImageElement

const TERMINAL_STATUSES = new Set(['completed', 'failed', 'cancelled', 'output_deleted'])
const PREVIEW_CACHE_DB_NAME = 'sub2api-batch-image-preview-cache'
const PREVIEW_CACHE_STORE_NAME = 'thumbnails'
const PREVIEW_THUMBNAIL_MAX_EDGE = 360
const PREVIEW_THUMBNAIL_QUALITY = 0.72
const PREVIEW_CACHE_MAX_AGE_MS = 3 * 24 * 60 * 60 * 1000
const PREVIEW_CACHE_MAX_ENTRIES = 120
const PREVIEW_CACHE_MAX_BYTES = 48 * 1024 * 1024
const BATCH_IMAGE_MAX_OUTPUTS_PER_ITEM = 4
const BATCH_IMAGE_MAX_OUTPUTS_PER_JOB = 200
const IMAGE_JOB_PREVIEW_LIMIT = 8
const imageRecordLimit = 50
const STORAGE_IMAGE_MODEL_KEY = 'creative-studio.selected.imageApiKeyId'
const STORAGE_VIDEO_MODEL_KEY = 'creative-studio.selected.videoApiKeyId'
const STORAGE_TEMPLATE_DRAWER_KEY = 'creative-studio.templateDrawerState'
const STORAGE_TEMPLATE_CATEGORY_KEY = 'creative-studio.templateCategory'
const outputCountOptions = Array.from({ length: BATCH_IMAGE_MAX_OUTPUTS_PER_ITEM }, (_, index) => index + 1)
const batchPageSizeOptions: SelectOption[] = [20, 50, 100].map(size => ({ value: size, label: String(size) }))
const videoDurationOptions = [5, 8, 10, 15]

const appStore = useAppStore()
const { copyToClipboard } = useClipboard()
const { t, locale } = useI18n()
const activeTab = ref<'image' | 'video'>('image')
const imageTool = ref<ImageCreativeTool>('text')
const templateDrawerState = ref<TemplateDrawerState>(readTemplateDrawerState())
const templateCategory = ref(readStoredString(STORAGE_TEMPLATE_CATEGORY_KEY, '全部'))
const showModelPicker = ref(false)
const creativePrompt = ref('')
const composerAspectRatio = ref('1:1')
const composerSize = ref('1K')
const composerCount = ref(1)

const creativeTemplates: CreativeTemplate[] = [
  {
    id: 'portrait-pro',
    title: '专业头像',
    description: '干净自然光，适合个人主页和工作头像。',
    mode: 'text',
    category: '头像',
    prompt: '一位年轻亚洲女性的专业商务头像，灰色背景，自然柔和光线，黑色西装，真实摄影质感，清晰五官，自信温和的表情，商业杂志封面风格，不添加文字、Logo、水印',
    aspectRatio: '1:1',
    size: '1K',
    count: 1,
    previewClass: 'template-preview-portrait',
  },
  {
    id: 'korean-id-photo',
    title: '韩系证件照',
    description: '清透妆感和柔和布光，社交平台很耐看。',
    mode: 'text',
    category: '头像',
    prompt: '韩系证件照风格的人像，年轻亚洲女性，浅灰白背景，柔和棚拍灯光，干净妆容，自然微笑，皮肤真实细腻，构图端正，高清摄影，不添加文字、Logo、水印',
    aspectRatio: '1:1',
    size: '1K',
    count: 1,
    previewClass: 'template-preview-id',
  },
  {
    id: 'magazine-cover',
    title: '杂志封面大片',
    description: '高级时装氛围，适合头像和朋友圈主图。',
    mode: 'text',
    category: '社交',
    prompt: '年轻亚洲男女的高级时尚杂志封面大片，城市天台黄昏光线，电影感构图，精致穿搭，真实摄影，浅景深，画面干净，不添加任何文字、Logo、水印',
    aspectRatio: '3:4',
    size: '1K',
    count: 1,
    previewClass: 'template-preview-magazine',
  },
  {
    id: 'travel-film',
    title: '旅行电影感',
    description: '海边、街头或山野都能变成故事感封面。',
    mode: 'text',
    category: '社交',
    prompt: '一张朋友圈旅行大片，年轻人站在海边公路旁，日落金色光线，胶片摄影质感，风吹发丝，远处海面闪光，真实自然，电影感构图，不添加文字、Logo、水印',
    aspectRatio: '4:5',
    size: '1K',
    count: 1,
    previewClass: 'template-preview-travel',
  },
  {
    id: 'cyber-avatar',
    title: '霓虹头像',
    description: '赛博光影和清晰五官，适合酷感头像。',
    mode: 'text',
    category: '头像',
    prompt: '赛博霓虹风格头像，年轻亚洲男性，夜晚城市霓虹反光，蓝紫与玫红光线，真实摄影和轻微未来感结合，清晰五官，背景有浅景深，不添加文字、Logo、水印',
    aspectRatio: '1:1',
    size: '1K',
    count: 1,
    previewClass: 'template-preview-cyber',
  },
  {
    id: 'product-poster',
    title: '电商产品海报',
    description: '干净高级的产品摄影，用于商品主图。',
    mode: 'text',
    category: '商品',
    prompt: '一瓶高端护肤精华放在浅色石材台面上，周围有水珠、绿叶和柔和晨光，干净高级的电商摄影，浅景深，真实产品广告质感，不添加文字、Logo、水印',
    aspectRatio: '4:5',
    size: '1K',
    count: 1,
    previewClass: 'template-preview-product',
  },
  {
    id: 'phone-wallpaper',
    title: '手机壁纸',
    description: '明亮治愈，适合竖屏壁纸和封面。',
    mode: 'text',
    category: '壁纸',
    prompt: '竖屏手机壁纸，清晨阳光穿过云层，柔和蓝色天空和浅粉色花海，画面干净治愈，有纵深感，细节丰富，不添加文字、Logo、水印',
    aspectRatio: '9:16',
    size: '1K',
    count: 1,
    previewClass: 'template-preview-wallpaper',
  },
  {
    id: 'cute-sticker',
    title: '可爱表情包',
    description: '软萌 3D 角色，适合社群和贴纸。',
    mode: 'text',
    category: '趣味',
    prompt: '一个可爱的 3D 表情包角色，圆润造型，开心挥手，浅色纯背景，柔和灯光，表情夸张但干净可爱，社交贴纸质感，不添加文字、Logo、水印',
    aspectRatio: '1:1',
    size: '1K',
    count: 1,
    previewClass: 'template-preview-sticker',
  },
  {
    id: 'background-replace',
    title: '背景替换',
    description: '保留主体，把背景换成明亮自然光场景。',
    mode: 'edit',
    category: '改图',
    prompt: '替换图片背景为明亮干净的自然光室内场景，保留主体人物和衣服细节，保持真实光影和边缘自然，不改变人物五官，不添加文字、Logo、水印',
    aspectRatio: '跟随原图',
    size: '1K',
    count: 1,
    previewClass: 'template-preview-bg',
  },
  {
    id: 'photo-to-comic',
    title: '照片转漫画',
    description: '保留人物特征，变成清爽漫画头像。',
    mode: 'edit',
    category: '改图',
    prompt: '将参考照片转换成清爽精致的漫画头像，保留人物发型、五官特征和表情，线条干净，颜色明亮，背景简洁，不添加文字、Logo、水印',
    aspectRatio: '1:1',
    size: '1K',
    count: 1,
    previewClass: 'template-preview-comic',
  },
  {
    id: 'pet-mascot',
    title: '宠物拟人',
    description: '把宠物变成可爱的社交头像角色。',
    mode: 'edit',
    category: '趣味',
    prompt: '将参考图中的宠物设计成可爱的 3D 吉祥物角色，保留毛色和主要特征，穿浅色连帽衫，明亮纯色背景，表情友好，有社交头像质感，不添加文字、Logo、水印',
    aspectRatio: '1:1',
    size: '1K',
    count: 1,
    previewClass: 'template-preview-pet',
  },
  {
    id: 'outfit-color',
    title: '衣服换色',
    description: '只改服装颜色，人物和背景保持自然。',
    mode: 'edit',
    category: '改图',
    prompt: '将参考图中人物的衣服颜色替换为柔和奶油白，保持衣服材质、褶皱和光影真实，不改变人物五官、姿势和背景，不添加文字、Logo、水印',
    aspectRatio: '跟随原图',
    size: '1K',
    count: 1,
    previewClass: 'template-preview-outfit',
  },
]

const columns = computed<Column[]>(() => [
  { key: 'select', label: '', sortable: false, class: 'w-12 text-center' },
  { key: 'id', label: t('batchImage.columns.taskName'), sortable: false, class: 'w-[240px] max-w-[240px]' },
  { key: 'model', label: t('batchImage.columns.model'), sortable: false, class: 'w-[180px] max-w-[180px] text-center' },
  { key: 'api_key_name', label: t('batchImage.columns.apiKey'), sortable: false, class: 'w-40 max-w-40 text-center' },
  { key: 'status', label: t('common.status'), sortable: false, class: 'w-28 text-center' },
  { key: 'counts', label: t('batchImage.columns.result'), sortable: false, class: 'w-32 text-center' },
  { key: 'cost', label: t('batchImage.columns.cost'), sortable: false, class: 'w-36 text-center' },
  { key: 'downloaded', label: t('batchImage.columns.downloadStatus'), sortable: false, class: 'w-40 text-center' },
  { key: 'actions', label: t('common.actions'), sortable: false, class: 'w-40 text-center' },
])

const statusFilterOptions = computed<SelectOption[]>(() => [
  { value: '', label: t('batchImage.filters.allStatuses') },
  { value: 'queued', label: t('batchImage.status.queued') },
  { value: 'running', label: t('batchImage.status.running') },
  { value: 'processing_results', label: t('batchImage.status.processingResults') },
  { value: 'settling', label: t('batchImage.status.settling') },
  { value: 'completed', label: t('batchImage.status.completed') },
  { value: 'failed', label: t('batchImage.status.failed') },
  { value: 'cancelled', label: t('batchImage.status.cancelled') },
  { value: 'output_deleted', label: t('batchImage.status.outputDeleted') },
])

const downloadFilterOptions = computed<SelectOption[]>(() => [
  { value: '', label: t('batchImage.filters.allDownloadStates') },
  { value: 'true', label: t('batchImage.filters.downloaded') },
  { value: 'false', label: t('batchImage.filters.notDownloaded') },
])

const form = reactive({
  apiKeyId: 0,
  taskName: '',
  model: '',
  responseMimeType: 'image/png',
})

const filters = reactive({
  taskName: '',
  apiKeyId: '',
  status: '',
  downloaded: '',
})

const pagination = reactive({
  page: 1,
  page_size: Math.min(getPersistedPageSize(20), 100),
  has_more: false,
})

const apiKeys = ref<ApiKey[]>([])
const loadingKeys = ref(false)
const loadingJobs = ref(false)
const submitting = ref(false)
const refreshing = ref(false)
const cancelling = ref(false)
const downloading = ref(false)
const downloadingBatchId = ref('')
const retryingBatchId = ref('')
const bulkDownloading = ref(false)
const bulkDeleting = ref(false)
const deletingBatchId = ref('')
const loadingItems = ref(false)
const loadingModels = ref(false)
const showCreateModal = ref(false)
const showGuideModal = ref(false)
const showVideoApiDocsModal = ref(false)
const showVideoPromptGuideModal = ref(false)
const currentJob = ref<BatchImageJob | null>(null)
const selectedBatchId = ref('')
const selectedBatchApiKeyId = ref(0)
const items = ref<BatchImageDetailItem[]>([])
const batchJobs = ref<BatchImageJobRow[]>([])
const selectedJobIds = ref(new Set<string>())
const expandedParentIds = ref(new Set<string>())
const promptRows = ref<PromptRow[]>([])
const promptDraft = ref('')
const customIdDraft = ref('')
const outputCountDraft = ref(1)
const referenceImageDrafts = ref<ReferenceImageDraft[]>([])
const videoFrameDraft = ref<VideoFrameDraft | null>(null)
const imageJobPreviewUrls = reactive<Record<string, string>>({})
const imageJobPreviewLoadingIds = ref(new Set<string>())
const itemPreviewUrls = reactive<Record<string, string>>({})
const previewLoadingIds = ref(new Set<string>())
const previewErrorIds = ref(new Set<string>())
const previewImageItem = ref<BatchImageDetailItem | null>(null)
const previewImageFullUrl = ref('')
const previewImageFullBlob = ref<Blob | null>(null)
const previewImageFullLoading = ref(false)
const previewImageFullError = ref('')
const previewImageDownloading = ref(false)
const availableBatchImageModels = ref<Array<{ value: string; label: string }>>([])
const modelLoadError = ref('')
const openMoreJobId = ref('')
const moreMenuStyle = ref<Record<string, string>>({})
const promptPopover = reactive({
  visible: false,
  text: '',
  style: {} as Record<string, string>,
})
let modelRequestSeq = 0
let imagePreviewRequestSeq = 0
let previewImageFullRequestSeq = 0
let pollTimer: ReturnType<typeof setInterval> | null = null
let previewCacheDBPromise: Promise<IDBDatabase | null> | null = null
let previewCacheCleanupTimer: ReturnType<typeof setInterval> | null = null
let promptPopoverCloseTimer: ReturnType<typeof setTimeout> | null = null
let promptPopoverOpenTimer: ReturnType<typeof setTimeout> | null = null
let activePromptPopoverTarget: HTMLElement | null = null

const geminiApiKeys = computed(() =>
  apiKeys.value.filter(isImageCreativeKey),
)

const grokApiKeys = computed(() =>
  apiKeys.value.filter(isVideoCreativeKey),
)

const selectedApiKey = computed(() =>
  geminiApiKeys.value.find((key) => key.id === Number(form.apiKeyId)) || null,
)

const selectedVideoApiKey = computed(() =>
  grokApiKeys.value.find((key) => key.id === Number(videoForm.apiKeyId)) || null,
)

const recentImageJobs = computed(() =>
  batchJobs.value
    .filter(job => !job.parent_batch_id)
    .slice(0, imageRecordLimit),
)

const templateCategories = computed(() => [
  '全部',
  ...Array.from(new Set(creativeTemplates.map(template => template.category))),
])

const filteredCreativeTemplates = computed(() => {
  if (templateCategory.value === '全部') return creativeTemplates
  return creativeTemplates.filter(template => template.category === templateCategory.value)
})

const aspectRatioOptions = computed(() => {
  if (activeTab.value === 'video') return ['16:9', '9:16', '1:1']
  if (imageTool.value === 'edit') return ['跟随原图', '1:1', '4:5', '3:4', '16:9', '9:16']
  return ['1:1', '4:5', '3:4', '16:9', '9:16']
})

const currentSizeOptions = computed(() =>
  activeTab.value === 'video' ? ['480p', '720p', '1080p'] : ['1K'],
)

const currentCountOptions = computed(() =>
  activeTab.value === 'video' ? [1] : outputCountOptions,
)

const selectedModelLabel = computed(() => {
  if (activeTab.value === 'video') {
    const key = selectedVideoApiKey.value
    return key ? `${videoForm.model} · ${key.name || `API Key #${key.id}`}` : '选择可用的视频模型'
  }
  const key = selectedApiKey.value
  return key ? `${form.model || '图片模型'} · ${key.name || `API Key #${key.id}`}` : '选择可用的图片模型'
})

const currentModelChoices = computed<ModelChoice[]>(() => {
  if (activeTab.value === 'video') {
    return apiKeys.value.map((key) => {
      const disabled = !isVideoCreativeKey(key)
      return {
        id: `video-${key.id}`,
        apiKeyId: key.id,
        label: `${videoModelForKey(key)} · ${key.name || `API Key #${key.id}`}`,
        maskedKey: maskApiKey(key.key),
        platformLabel: platformLabel(key),
        selected: !disabled && Number(videoForm.apiKeyId) === key.id,
        disabled,
        reason: disabled ? creativeKeyUnavailableReason(key, 'video') : '',
        mode: 'video',
      }
    })
  }
  return apiKeys.value.map((key) => {
    const disabled = !isImageCreativeKey(key)
    return {
      id: `image-${key.id}`,
      apiKeyId: key.id,
      label: `${Number(form.apiKeyId) === key.id && form.model ? form.model : '图片模型'} · ${key.name || `API Key #${key.id}`}`,
      maskedKey: maskApiKey(key.key),
      platformLabel: platformLabel(key),
      selected: !disabled && Number(form.apiKeyId) === key.id,
      disabled,
      reason: disabled ? creativeKeyUnavailableReason(key, 'image') : '',
      mode: 'image',
    }
  })
})

const creativePromptPlaceholder = computed(() => {
  if (activeTab.value === 'video') return '描述你想生成的镜头，比如：雨后街头，一个人撑伞走过霓虹招牌，镜头缓慢推进。'
  if (imageTool.value === 'edit') return '告诉我想怎么改这张图，比如：把背景换成自然光咖啡馆，保留人物五官和衣服细节。'
  return '描述你想看到的画面，比如：一张干净明亮的护肤品海报，玻璃瓶旁有水珠和绿叶。'
})

const creativeComposerHint = computed(() => {
  if (activeTab.value === 'video') return `视频任务最多保留 ${videoLimits.retentionDays} 天，请在完成后及时下载。`
  if (!selectedApiKey.value) return '还没有可用于图片创作的 API Key，配置好后这里就能开始生成。'
  if (imageTool.value === 'edit') return '改图会尽量保留参考图主体，具体效果取决于所选模型能力。'
  return '生成结果会进入任务记录，完成后可以预览或下载。'
})

const creativeSubmittingDisabled = computed(() => {
  if (activeTab.value === 'video') {
    return videoSubmitting.value || !selectedVideoApiKey.value || !creativePrompt.value.trim()
  }
  return submitting.value || (loadingModels.value && !form.model) || !selectedApiKey.value || !form.model || !creativePrompt.value.trim() || (imageTool.value === 'edit' && referenceImageDrafts.value.length === 0)
})

const videoTasks = ref<CreativeVideoTask[]>([])
const videoForm = reactive({
  apiKeyId: 0,
  model: 'grok-imagine-video',
  prompt: '',
  resolution: '720p',
  duration: 8,
})
const videoSubmitting = ref(false)
const videoLoadingTasks = ref(false)
const videoDownloadingId = ref('')
const videoPreviewingId = ref('')
const videoDeletingId = ref('')
const videoPreviewUrl = ref('')
const videoPreviewTitle = ref('视频预览')
const videoPreviewError = ref('')
const videoDetailTask = ref<CreativeVideoTask | null>(null)
const videoTaskKeyMap = reactive<Record<string, string>>({})
const videoThumbnailUrls = reactive<Record<string, string>>({})
const videoThumbnailLoadingIds = reactive(new Set<string>())
const videoThumbnailFailedIds = reactive(new Set<string>())
const videoNow = ref(Date.now())
const videoLimits = reactive({
  retentionDays: 3,
  maxRecords: 50,
  maxRunning: 5,
})
let videoPollTimer: ReturnType<typeof setInterval> | null = null
let videoElapsedTimer: ReturnType<typeof setInterval> | null = null

const videoRunningCount = computed(() =>
  videoTasks.value.filter(task => isCreativeVideoProcessing(task.status)).length,
)

const videoDetailAspectRatio = computed(() => {
  const task = videoDetailTask.value
  if (!task) return '-'
  if (task.resolution && String(task.resolution).toLowerCase().includes('768')) return '16:9'
  return composerAspectRatio.value || '16:9'
})

const filteredApiKeys = computed(() => {
  const selectedFilterID = Number(filters.apiKeyId || 0)
  if (!selectedFilterID) return geminiApiKeys.value
  return geminiApiKeys.value.filter(key => key.id === selectedFilterID)
})

const apiKeyFilterOptions = computed<SelectOption[]>(() => [
  { value: '', label: t('batchImage.filters.allApiKeys') },
  ...geminiApiKeys.value.map(key => ({
    value: String(key.id),
    label: key.name || `API Key #${key.id}`,
  })),
])

const selectedRows = computed(() =>
  batchJobs.value.filter(job => selectedJobIds.value.has(job.id)),
)

const childrenByParent = computed(() => {
  const groups = new Map<string, BatchImageJobRow[]>()
  for (const job of batchJobs.value) {
    if (!job.parent_batch_id) continue
    const rows = groups.get(job.parent_batch_id) || []
    rows.push(job)
    groups.set(job.parent_batch_id, rows)
  }
  for (const rows of groups.values()) {
    rows.sort((a, b) => a.created_at - b.created_at)
  }
  return groups
})

const visibleBatchJobs = computed(() => {
  const rows: BatchImageJobRow[] = []
  for (const job of batchJobs.value.filter(item => !item.parent_batch_id)) {
    rows.push(job)
    if (expandedParentIds.value.has(job.id)) {
      rows.push(...(childrenByParent.value.get(job.id) || []).map(child => ({ ...child, is_child: true })))
    }
  }
  return rows
})

const selectedDownloadableRows = computed(() =>
  selectedRows.value.filter(job => canDownload(job)),
)

const allVisibleSelected = computed(() =>
  visibleBatchJobs.value.length > 0 && visibleBatchJobs.value.every(job => selectedJobIds.value.has(job.id)),
)

const someVisibleSelected = computed(() =>
  visibleBatchJobs.value.some(job => selectedJobIds.value.has(job.id)) && !allVisibleSelected.value,
)

const previewImageUrl = computed(() => {
  const item = previewImageItem.value
  if (!item) return ''
  return itemPreviewUrls[itemPreviewKey(item)] || ''
})

const previewImageDisplayUrl = computed(() => previewImageFullUrl.value || previewImageUrl.value)

const previewImageCandidates = computed(() =>
  items.value.filter(item => canLoadItemPreview(item)),
)

const previewImageJob = computed(() => {
  const item = previewImageItem.value
  if (!item) return null
  const batchId = item.batch_id || selectedBatchId.value || currentJob.value?.id || ''
  return batchJobs.value.find(job => job.id === batchId) ||
    (currentJob.value && currentJob.value.id === batchId ? toJobRow(currentJob.value, keyForSelectedBatch() || selectedApiKey.value) : null)
})

const previewImageJobModel = computed(() => previewImageJob.value?.model || currentJob.value?.model || '-')

const previewImageJobCost = computed(() => {
  const job = previewImageJob.value || currentJob.value
  return job ? costLabel(job) : '-'
})

const previewImageCreatedAt = computed(() => {
  const job = previewImageJob.value || currentJob.value
  return job?.created_at ? formatDate(job.created_at) : '-'
})

const previewImageIndexText = computed(() => {
  const item = previewImageItem.value
  const candidates = previewImageCandidates.value
  if (!item || candidates.length <= 1) return t('batchImage.imagePreview.singleImage')
  const index = candidates.findIndex(candidate => itemPreviewKey(candidate) === itemPreviewKey(item))
  return t('batchImage.imagePreview.imageIndex', {
    index: index >= 0 ? index + 1 : 1,
    count: candidates.length,
  })
})

const previewImageFormatText = computed(() => {
  const item = previewImageItem.value
  const fromBlob = previewImageFullBlob.value?.type || ''
  const mime = String(fromBlob || item?.mime_type || '').trim()
  const ext = String(item?.file_extension || imageExtensionFromMime(mime) || '').replace(/^\./, '').toUpperCase()
  if (mime && ext) return `${ext} · ${mime}`
  return ext || mime || '-'
})

const previewImageFileSizeText = computed(() => formatImageFileSize(previewImageFullBlob.value?.size || 0))

const recoveredOriginalCustomIds = computed(() => {
  const rootBatchId = detailRootBatchId()
  if (!rootBatchId) return new Set<string>()
  const ids = new Set<string>()
  for (const item of items.value) {
    if (!isChildDetailItem(item) || !isSuccessfulImageItem(item)) continue
    const sourceCustomID = retrySourceCustomID(item.custom_id)
    if (sourceCustomID) ids.add(sourceCustomID)
  }
  return ids
})

const currentDisplayJob = computed(() => {
  if (!currentJob.value) return null
  return displayJob(currentJob.value)
})

const endpointBase = computed(() => {
  const configured = appStore.apiBaseUrl?.trim()
  if (configured) return configured.replace(/\/+$/, '')
  if (typeof window !== 'undefined') return window.location.origin.replace(/\/+$/, '')
  return '<你的 Sub2API API 端点>'
})

const apiBaseURL = computed(() => endpointBase.value)

const videoApiDocsText = computed(() => `# 视频 API 文档

## 接入说明

服务地址：${joinEndpointPath(apiBaseURL.value, '/v1')}

所有请求均需携带 API 密钥：

Authorization: Bearer YOUR_API_KEY
Content-Type: application/json

当前真实开放能力：
- Grok 文生视频 / 首帧图生视频
- OpenAI 视频模型
- MiniMax-H3 文生视频 / 首帧图生视频

MiniMax-H3 的视频、音频混合参考能力暂未在本接口开放。

## 接口

- GET /v1/models：获取当前密钥可用模型
- POST /v1/videos/generations：创建视频生成任务
- GET /v1/videos/tasks：查询最近任务，支持 limit
- GET /v1/videos/{request_id}：查询任务状态
- GET /v1/videos/{request_id}/content：下载视频文件流

## 创建任务

通用写法：

curl '${joinEndpointPath(apiBaseURL.value, '/v1/videos/generations')}' \\
  -H 'Authorization: Bearer YOUR_API_KEY' \\
  -H 'Content-Type: application/json' \\
  -d '{
    "model": "MiniMax-H3",
    "prompt": "清晨的海边，镜头缓缓向前推进，阳光洒在海面",
    "duration": 8,
    "resolution": "768P",
    "aspect_ratio": "16:9"
  }'

MiniMax-H3 content 写法：

curl '${joinEndpointPath(apiBaseURL.value, '/v1/videos/generations')}' \\
  -H 'Authorization: Bearer YOUR_API_KEY' \\
  -H 'Content-Type: application/json' \\
  -d '{
    "model": "MiniMax-H3",
    "content": [
      { "type": "text", "text": "让画面中的人物微笑并缓缓转身，镜头保持稳定" },
      { "type": "image_url", "role": "first_frame", "image_url": { "url": "https://assets.example.com/reference.jpg" } }
    ],
    "duration": 8,
    "resolution": "768P",
    "ratio": "16:9"
  }'

## 查询与下载

curl '${joinEndpointPath(apiBaseURL.value, '/v1/videos/YOUR_REQUEST_ID')}' \\
  -H 'Authorization: Bearer YOUR_API_KEY'

curl '${joinEndpointPath(apiBaseURL.value, '/v1/videos/YOUR_REQUEST_ID/content')}' \\
  -H 'Authorization: Bearer YOUR_API_KEY' \\
  --fail --output video.mp4

## 状态与保留

- pending/running：处理中
- done/completed：完成
- failed：失败
- expired：过期

本站任务记录默认保留 ${videoLimits.retentionDays} 天，超过 ${videoLimits.maxRecords} 条会提前清理最早记录。视频文件有效期还受上游供应商限制，完成后请及时下载。`)

const selectedModelReferenceLimit = computed(() => referenceImageLimitForModel(form.model))

const estimatedOutputCount = computed(() =>
  promptRows.value.reduce((sum, row) => sum + normalizeOutputCount(row.output_count), 0),
)

const parsedItems = computed<BatchImageSubmitItem[]>(() => {
  const used = new Set<string>()
  return promptRows.value
    .map((row, index) => {
      const customID = uniqueCustomID(row.custom_id || `img_${String(index + 1).padStart(3, '0')}`, used, index)
      const item: BatchImageSubmitItem = { custom_id: customID, prompt: row.prompt.trim() }
      const outputCount = normalizeOutputCount(row.output_count)
      if (outputCount > 1) {
        item.output_count = outputCount
      }
      if (row.reference_images.length) {
        item.reference_images = row.reference_images
      }
      return item
    })
    .filter(item => item.prompt)
})

function readStoredString(key: string, fallback: string) {
  if (typeof window === 'undefined') return fallback
  return window.localStorage.getItem(key) || fallback
}

function readStoredNumber(key: string) {
  if (typeof window === 'undefined') return 0
  const value = Number(window.localStorage.getItem(key) || 0)
  return Number.isFinite(value) ? value : 0
}

function writeStoredString(key: string, value: string) {
  if (typeof window === 'undefined') return
  window.localStorage.setItem(key, value)
}

function readTemplateDrawerState(): TemplateDrawerState {
  const value = readStoredString(STORAGE_TEMPLATE_DRAWER_KEY, 'rail')
  return value === 'collapsed' || value === 'expanded' || value === 'rail' ? value : 'rail'
}

function isImageCreativeKey(key: ApiKey) {
  const platform = key.group?.platform || ''
  return key.status === 'active' &&
    ['gemini', 'openai', 'minimax'].includes(platform) &&
    key.group?.allow_batch_image_generation === true
}

function isVideoCreativeKey(key: ApiKey) {
  const platform = key.group?.platform || ''
  return key.status === 'active' &&
    ['grok', 'openai', 'minimax'].includes(platform) &&
    key.group?.allow_image_generation === true
}

function creativeKeyUnavailableReason(key: ApiKey, mode: 'image' | 'video') {
  if (key.status !== 'active') return '当前 API Key 未启用'
  const platform = key.group?.platform || ''
  if (mode === 'image') {
    if (!['gemini', 'openai', 'minimax'].includes(platform)) return '该平台的图片创作能力暂未接入当前创作台'
    if (key.group?.allow_batch_image_generation !== true) return '所属分组未开启创作台图片能力'
    return '暂不可用于图片创作'
  }
  if (!['grok', 'openai', 'minimax'].includes(platform)) return '该平台的视频创作能力暂未接入当前创作台'
  if (key.group?.allow_image_generation !== true) return '所属分组未开启创作台视频能力'
  return '暂不可用于视频创作'
}

function platformLabel(key: ApiKey) {
  const platform = String(key.group?.platform || '').trim()
  const group = String(key.group?.name || '').trim()
  if (group && platform) return `${group} / ${platform}`
  return group || platform || '平台'
}

function maskApiKey(value: string | null | undefined) {
  const raw = String(value || '').trim()
  if (!raw) return '****'
  const tail = raw.slice(-4)
  return `**** ${tail}`
}

function videoModelForKey(key: ApiKey) {
  const platform = key.group?.platform || ''
  if (platform === 'openai') return 'sora-2'
  if (platform === 'minimax') return 'MiniMax-H3'
  return 'grok-imagine-video'
}

function imageModelForKey(key: ApiKey) {
  const platform = key.group?.platform || ''
  if (platform === 'openai') return 'gpt-image-2'
  if (platform === 'minimax') return 'image-01'
  return 'gemini-2.5-flash-image'
}

function selectDefaultCreativeModels() {
  const savedImageId = readStoredNumber(STORAGE_IMAGE_MODEL_KEY)
  const savedVideoId = readStoredNumber(STORAGE_VIDEO_MODEL_KEY)
  const savedImage = geminiApiKeys.value.find(key => key.id === savedImageId)
  const savedVideo = grokApiKeys.value.find(key => key.id === savedVideoId)

  if (savedImage) {
    form.apiKeyId = savedImage.id
    if (!form.model) form.model = imageModelForKey(savedImage)
  } else if (!selectedApiKey.value && geminiApiKeys.value.length > 0) {
    form.apiKeyId = geminiApiKeys.value[0].id
    form.model = imageModelForKey(geminiApiKeys.value[0])
    if (savedImageId) appStore.showError(`上次选择的 API Key 当前不可用，已为你切换到 ${geminiApiKeys.value[0].name || '可用模型'}。`)
  }

  if (savedVideo) {
    videoForm.apiKeyId = savedVideo.id
    videoForm.model = videoModelForKey(savedVideo)
  } else if (!selectedVideoApiKey.value && grokApiKeys.value.length > 0) {
    videoForm.apiKeyId = grokApiKeys.value[0].id
    videoForm.model = videoModelForKey(grokApiKeys.value[0])
    if (savedVideoId) appStore.showError(`上次选择的 API Key 当前不可用，已为你切换到 ${grokApiKeys.value[0].name || '可用模型'}。`)
  }
}

function switchCreativeMode(mode: 'image' | 'video') {
  activeTab.value = mode
  composerAspectRatio.value = mode === 'video' ? '16:9' : imageTool.value === 'edit' ? '跟随原图' : '1:1'
  composerSize.value = mode === 'video' ? '720p' : '1K'
  composerCount.value = 1
}

function cycleTemplateDrawer() {
  templateDrawerState.value = templateDrawerState.value === 'collapsed'
    ? 'rail'
    : templateDrawerState.value === 'rail'
      ? 'expanded'
      : 'collapsed'
}

function applyCreativeTemplate(template: CreativeTemplate) {
  activeTab.value = 'image'
  imageTool.value = template.mode
  creativePrompt.value = template.prompt
  composerAspectRatio.value = template.aspectRatio
  composerSize.value = template.size
  composerCount.value = template.count
  if (template.mode === 'edit') {
    appStore.showSuccess('已切换到改图，请先上传参考图。')
  }
}

function selectModelChoice(choice: ModelChoice) {
  if (choice.disabled) return
  if (choice.mode === 'image') {
    form.apiKeyId = choice.apiKeyId
    const key = geminiApiKeys.value.find(item => item.id === choice.apiKeyId)
    if (key) form.model = imageModelForKey(key)
    writeStoredString(STORAGE_IMAGE_MODEL_KEY, String(choice.apiKeyId))
  } else {
    videoForm.apiKeyId = choice.apiKeyId
    const key = grokApiKeys.value.find(item => item.id === choice.apiKeyId)
    if (key) videoForm.model = videoModelForKey(key)
    writeStoredString(STORAGE_VIDEO_MODEL_KEY, String(choice.apiKeyId))
  }
  showModelPicker.value = false
}

async function handleSingleEditReferenceImage(event: Event) {
  referenceImageDrafts.value = []
  await handleReferenceImageFiles(event)
}

async function handleVideoFrameImage(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  if (!['image/png', 'image/jpeg', 'image/webp'].includes(file.type)) {
    appStore.showError('首帧图仅支持 PNG、JPG、WebP 格式')
    return
  }
  if (file.size > 10 * 1024 * 1024) {
    appStore.showError('首帧图不能超过 10MB')
    return
  }
  try {
    const data = await readFileAsBase64(file)
    videoFrameDraft.value = {
      name: file.name,
      mimeType: file.type,
      size: file.size,
      data,
    }
  } catch {
    appStore.showError('首帧图读取失败，请重新选择')
  }
}

async function submitCreative() {
  if (activeTab.value === 'video') {
    videoForm.prompt = creativePrompt.value.trim()
    videoForm.resolution = composerSize.value
    const submitted = await submitVideo()
    if (submitted) creativePrompt.value = ''
    return
  }

  promptRows.value = []
  promptDraft.value = creativePrompt.value.trim()
  outputCountDraft.value = composerCount.value
  form.taskName = imageTool.value === 'edit' ? 'AI 改图' : 'AI 生图'
  form.responseMimeType = 'image/png'
  const submitted = await submitJob()
  if (submitted) {
    creativePrompt.value = ''
    referenceImageDrafts.value = []
  }
}

function referenceImageLimitForModel(model: string) {
  const normalized = String(model || '').toLowerCase()
  if (normalized.includes('pro-image')) return 14
  if (normalized.includes('flash-image')) return 3
  if (normalized.startsWith('gpt-image-')) return 4
  if (normalized.includes('image-01') || normalized.includes('image-')) return 4
  return 0
}

const agentInstruction = computed(() => `---
name: sub2api-batch-image
description: 当用户希望用 Gemini/Vertex 批量生成图片、批量跑提示词、下载批量生图结果、重试失败图片时使用。
---

你是 Codex 中的批量生图执行 Agent。用户不需要手动填写页面表单；你应从当前聊天、用户给的文件、目录或上下文中整理任务名称、prompt 列表和输出目录，只有缺少关键决策时才向用户提问。

默认端点：
${endpointBase.value}

你需要自己完成：
1. 从用户聊天或附件中提取 prompt。每条 prompt 保留完整文本，按顺序生成稳定 custom_id，例如 img_001、img_002。
2. 从用户要求或上下文推断任务名称；没有明确名称时用当前时间生成任务名。
3. 从用户要求或上下文推断输出目录；如果用户没有说保存到哪里，才询问用户。
4. 提交前必须先计算 expected_output_count = 所有 item 的 output_count 之和。单个批量任务硬性最多 200 张输出图；超过 200 张必须拆成多组任务，不能提交一个超大任务，也不能把参考图附件上限当成生成张数上限。
5. 如果用户提供参考图，把参考图按用途绑定到具体 item。参考图只是输入附件，不是输出图数量。模型单条限制必须按模型执行：Gemini 2.5 Flash Image 每条最多 3 张参考图；Gemini 3 Pro Image 每条最多 14 张参考图。不要把后端附件风控理解成 Pro 单条能力：按 output_count 展开后，所有 item 的参考图附件总数还有内部保护阈值 1000 个，inline base64 参考图解码后总量最多 128MB。这个 1000 只是服务器拒绝异常请求的保护阈值，不是推荐规模；参考图很多或总请求体较大时应主动拆分任务。
6. 参考图会按 output_count 重复消耗输入 token；大量任务、重复复用同一张参考图或参考图总体积较大时，优先使用 gs:// file_uri 或拆分成多组任务。
7. 选择 API Key 和模型：先获取当前可用的批量生图 Key/模型；如果用户指定模型且该 Key 支持，则使用用户指定模型；否则使用该 Key 可用模型中的默认/第一个。不要展示或询问内部 provider 名称。
8. 调用批量生图 API 提交、轮询、下载，不要求用户去页面里手填。

API 调用规范：
- 模型：GET ${joinEndpointPath(endpointBase.value, '/v1/images/batches/models')}
- 提交：POST ${joinEndpointPath(endpointBase.value, '/v1/images/batches')}
- 查询：GET ${joinEndpointPath(endpointBase.value, '/v1/images/batches/{id}')}
- 明细：GET ${joinEndpointPath(endpointBase.value, '/v1/images/batches/{id}/items')}
- 下载：GET ${joinEndpointPath(endpointBase.value, '/v1/images/batches/{id}/download')}
- 取消：POST ${joinEndpointPath(endpointBase.value, '/v1/images/batches/{id}/cancel')}

提交请求体：
{
  "model": "<按所选 Key 可用模型填写>",
  "task_name": "<从聊天推断；为空则用当前时间>",
  "image_size": "1K",
  "response_mime_type": "image/png",
  "items": [
    {
      "custom_id": "img_001",
      "prompt": "<第一条完整 prompt>",
      "output_count": 1,
      "reference_images": [
        {
          "id": "face",
          "type": "subject",
          "mime_type": "image/png",
          "data": "<base64，不含 data:image/png;base64, 前缀>"
        }
      ]
    }
  ]
}

必须遵守：
- 不要把 API Key 写入仓库、日志、提交记录或最终回复。
- 不要把参考图 base64 写入最终回复、日志或公开文件。恢复记录中只保存参考图文件名、用途、数量和请求 JSON 文件路径；若请求 JSON 文件包含 base64，应保存在用户指定输出目录且不要提交到仓库。
- output_count 表示同一 prompt 和参考图重复生成几张，默认 1，每条最多 4；这不是依赖 Gemini 单次请求返回多图，而是系统展开成多个真实任务项。提交前必须确认预计输出图总数不超过 200，超过就拆分成多组任务。绝不能因为参考图附件有更高的内部保护阈值，就提交会生成超过 200 张图的任务。
- 当前对用户的批量生图计费仍按成功输出图片数量结算，不单独对参考图加价。可以向用户说明：参考图会产生少量上游输入 token 和临时存储成本，且会随 output_count 重复计算；页面显示的冻结/结算金额按输出图片数量计算。
- 提交成功后，必须立刻在输出目录写入本地恢复记录，例如 batch-image-resume.json。不要在恢复记录里保存 API Key。
- 恢复记录至少包含：endpoint、task_name、batch_id、model、output_dir、request_file、submitted_at、last_status、status_url、items_url、download_url、prompt_count、expected_output_count，以及可用于失败重试的 custom_id 到 prompt 映射或请求 JSON 文件路径。
- 每次查询状态后更新恢复记录，写入 last_checked_at、last_status、成功数、失败数、实际扣费和失败摘要。会话中断或暂停后，下次必须能凭该文件继续查询、下载或重试。
- 不要高频轮询。首次查询等待约 20 到 30 秒；queued 状态每 60 到 120 秒查询一次；如果连续 3 次仍是 queued，就先停止主动查询，告诉用户任务仍在排队，并保留恢复记录，之后可继续其他任务或等待用户稍后让你恢复。
- running 状态每约 60 秒查询一次，服务器压力大或大批量任务时可以更久；processing_results 等接近完成的状态可每 20 到 45 秒查询一次。
- 任务完成后报告任务名、任务 id、成功数、失败数、实际扣费和保存路径。
- 只下载成功图片。部分失败时，先展示失败 custom_id、错误码、错误来源和简要原因。
- 重试只能重试失败项，不能重复提交已成功项。若历史任务没有保存失败项 prompt，必须告诉用户无法自动重试，并询问用户是否提供原 prompt。
- 取消任务前必须提醒：已被系统索引为成功的图片仍会按成功项结算扣费，其余冻结金额会释放。
- 图片预览按需加载；不要为了查看列表自动批量加载图片内容。`)

function joinEndpointPath(base: string, path: string): string {
  return `${base.replace(/\/+$/, '')}/${path.replace(/^\/+/, '')}`
}

function uniqueCustomID(raw: string, used: Set<string>, index: number): string {
  const base = raw.replace(/[^\w.-]+/g, '_').replace(/^_+|_+$/g, '') || `img_${String(index + 1).padStart(3, '0')}`
  let candidate = base
  let suffix = 2
  while (used.has(candidate)) {
    candidate = `${base}_${suffix}`
    suffix += 1
  }
  used.add(candidate)
  return candidate
}

function normalizeOutputCount(value: unknown): number {
  const parsed = Math.floor(Number(value || 1))
  if (!Number.isFinite(parsed)) return 1
  return Math.min(BATCH_IMAGE_MAX_OUTPUTS_PER_ITEM, Math.max(1, parsed))
}

function addPromptRow() {
  const prompt = promptDraft.value.trim()
  if (!prompt) return
  const outputCount = normalizeOutputCount(outputCountDraft.value)
  const used = new Set(promptRows.value.map(row => row.custom_id))
  const customID = uniqueCustomID(customIdDraft.value || `img_${String(promptRows.value.length + 1).padStart(3, '0')}`, used, promptRows.value.length)
  promptRows.value = [
    ...promptRows.value,
    {
      localId: `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
      custom_id: customID,
      prompt,
      output_count: outputCount,
      reference_images: referenceImageDrafts.value.map(({ name: _name, size: _size, ...ref }) => ref),
    },
  ]
  promptDraft.value = ''
  customIdDraft.value = ''
  outputCountDraft.value = 1
  referenceImageDrafts.value = []
}

function removePromptRow(index: number) {
  promptRows.value = promptRows.value.filter((_, currentIndex) => currentIndex !== index)
}

function removeReferenceImageDraft(index: number) {
  referenceImageDrafts.value = referenceImageDrafts.value.filter((_, currentIndex) => currentIndex !== index)
}

async function handleReferenceImageFiles(event: Event) {
  const input = event.target as HTMLInputElement
  const files = Array.from(input.files || [])
  input.value = ''
  if (files.length === 0) return
  const limit = selectedModelReferenceLimit.value
  if (limit <= 0) {
    appStore.showError(t('batchImage.create.modelNoReferenceImages'))
    return
  }
  const slots = Math.max(0, limit - referenceImageDrafts.value.length)
  if (slots <= 0) {
    appStore.showError(t('batchImage.create.refLimitReached', { limit }))
    return
  }
  const accepted = files.slice(0, slots)
  if (accepted.length < files.length) {
    appStore.showError(t('batchImage.create.refLimitExceededIgnored', { limit }))
  }
  const next: ReferenceImageDraft[] = []
  for (const file of accepted) {
    if (!['image/png', 'image/jpeg', 'image/webp'].includes(file.type)) {
      appStore.showError(t('batchImage.create.refFormatUnsupported'))
      continue
    }
    if (file.size > 10 * 1024 * 1024) {
      appStore.showError(t('batchImage.create.refFileTooLarge', { name: file.name }))
      continue
    }
    const data = await readFileAsBase64(file)
    next.push({
      id: file.name,
      type: 'reference',
      mime_type: file.type,
      data,
      name: file.name,
      size: file.size,
    })
  }
  referenceImageDrafts.value = [...referenceImageDrafts.value, ...next]
}

function readFileAsBase64(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onerror = () => reject(reader.error || new Error('Failed to read file'))
    reader.onload = () => {
      const result = String(reader.result || '')
      resolve(result.includes(',') ? result.slice(result.indexOf(',') + 1) : result)
    }
    reader.readAsDataURL(file)
  })
}

async function loadApiKeys() {
  loadingKeys.value = true
  try {
    const response = await keysAPI.list(1, 100, { sort_by: 'created_at', sort_order: 'desc' })
    apiKeys.value = response.items || []
    selectDefaultCreativeModels()
    if (filters.apiKeyId && !geminiApiKeys.value.some(key => String(key.id) === filters.apiKeyId)) {
      filters.apiKeyId = ''
    }
    if (!selectedApiKey.value) {
      availableBatchImageModels.value = []
      form.model = ''
    }
  } catch (error: any) {
    appStore.showError(batchImageErrorMessage(error, batchImageText('loadKeysFailed')))
  } finally {
    loadingKeys.value = false
  }
}

async function loadAvailableModels() {
  const key = selectedApiKey.value
  const requestID = ++modelRequestSeq
  modelLoadError.value = ''
  availableBatchImageModels.value = []
  if (!key) {
    form.model = ''
    return
  }
  if (!form.model) {
    form.model = imageModelForKey(key)
  }

  loadingModels.value = true
  try {
    const result = await listBatchImageModels(key.key)
    if (requestID !== modelRequestSeq) return
    const seen = new Set<string>()
    availableBatchImageModels.value = (result.data || [])
      .map(model => String(model.id || '').trim())
      .filter((model) => {
        if (!model || seen.has(model)) return false
        seen.add(model)
        return true
      })
      .map(model => ({ value: model, label: model }))
    if (availableBatchImageModels.value.length > 0 && !availableBatchImageModels.value.some(model => model.value === form.model)) {
      form.model = availableBatchImageModels.value[0]?.value || imageModelForKey(key)
    }
  } catch (error: any) {
    if (requestID !== modelRequestSeq) return
    modelLoadError.value = batchImageErrorMessage(error, batchImageText('loadModelsFailed'))
  } finally {
    if (requestID === modelRequestSeq) {
      loadingModels.value = false
    }
  }
}

async function refreshPage() {
  await loadApiKeys()
  if (activeTab.value === 'video') {
    await loadVideoTasks()
  } else {
    await loadBatchJobs()
  }
}

function apiKeyValueById(id: number) {
  return apiKeys.value.find(key => key.id === Number(id))?.key || ''
}

function creativeVideoRequestId(response: any) {
  for (const path of ['request_id', 'id', 'data.request_id', 'data.id', 'video.request_id', 'video.id', 'task_id']) {
    const value = path.split('.').reduce<any>((current, part) => current?.[part], response)
    if (value) return String(value)
  }
  return ''
}

async function loadVideoTasks() {
  if (!grokApiKeys.value.length) {
    videoTasks.value = []
    videoLimits.retentionDays = 3
    videoLimits.maxRecords = 50
    videoLimits.maxRunning = 5
    return
  }
  videoLoadingTasks.value = true
  try {
    const rows: CreativeVideoTask[] = []
    let limitsLoaded = false
    for (const key of grokApiKeys.value) {
      try {
        const result = await listCreativeVideoTasks(key.key, 500)
        if (!limitsLoaded) {
          videoLimits.retentionDays = Number(result.retention_days) || 3
          videoLimits.maxRecords = Number(result.max_records_per_user) || 50
          videoLimits.maxRunning = Number(result.max_running_per_user) || 5
          limitsLoaded = true
        }
        for (const task of result.data || []) {
          const existing = videoTasks.value.find(row => row.id === task.id)
          const existingStatus = existing?.status || ''
          if (existing && isCreativeVideoCompleted(existingStatus) && isCreativeVideoProcessing(task.status)) {
            task.status = existingStatus
            task.duration_seconds = task.duration_seconds || existing.duration_seconds
          }
          rows.push(task)
          videoTaskKeyMap[task.id] = key.key
        }
      } catch {
        // One key failing should not hide tasks from other keys.
      }
    }
    const seen = new Set<string>()
    videoTasks.value = rows
      .filter((task) => {
        if (!task.id || seen.has(task.id)) return false
        seen.add(task.id)
        return true
      })
      .sort((a, b) => Number(b.created_at || 0) - Number(a.created_at || 0))
      .slice(0, videoLimits.maxRecords)
    scheduleVideoThumbnails()
    manageVideoPolling()
  } catch (error: any) {
    appStore.showError(error?.message || '加载视频任务失败')
  } finally {
    videoLoadingTasks.value = false
  }
}

async function submitVideo(): Promise<boolean> {
  const apiKey = apiKeyValueById(videoForm.apiKeyId)
  if (!apiKey || !videoForm.prompt.trim()) return false
  videoSubmitting.value = true
  try {
    const payload = {
      model: videoForm.model,
      prompt: videoForm.prompt.trim(),
      aspect_ratio: composerAspectRatio.value,
      resolution: videoForm.resolution,
      duration: videoForm.duration,
      ...(videoFrameDraft.value
        ? {
            image: {
              type: 'image_url',
              url: `data:${videoFrameDraft.value.mimeType};base64,${videoFrameDraft.value.data}`,
            },
          }
        : {}),
    }
    const response = await createCreativeVideo(apiKey, {
      ...payload,
    })
    const requestId = creativeVideoRequestId(response)
    if (requestId) videoTaskKeyMap[requestId] = apiKey
    videoForm.prompt = ''
    videoFrameDraft.value = null
    appStore.showSuccess('视频任务已提交')
    await loadVideoTasks()
    return true
  } catch (error: any) {
    appStore.showError(creativeVideoSubmitErrorMessage(error))
    return false
  } finally {
    videoSubmitting.value = false
  }
}

function creativeVideoSubmitErrorMessage(error: any) {
  const code = String(error?.code || '').trim()
  const message = String(error?.message || '').trim()
  if (code === 'CREATIVE_VIDEO_RUNNING_LIMIT_EXCEEDED' || message.includes('too many running creative video tasks')) {
    return `已有 ${videoLimits.maxRunning} 个视频任务在生成中。系统已尝试刷新旧任务状态，请稍后重试；如果旧任务已经完成，可以刷新任务记录后再提交。`
  }
  if (code === 'CREATIVE_VIDEO_DISABLED') return '视频创作能力暂未开启，请检查创作台配置。'
  if (code === 'CREATIVE_VIDEO_TASK_PERSISTENCE_FAILED') return '视频任务记录保存失败，请先让管理员完成数据库迁移后再重试。'
  if (code === 'video_no_eligible_account') return '当前视频分组没有可用上游账号，请检查账号状态或额度。'
  if (code === 'upstream_error') return '上游视频服务提交失败，请稍后重试或检查上游 Key / Base URL。'
  return message || '提交视频任务失败'
}

async function refreshRunningVideos() {
  const running = videoTasks.value.filter(task => isCreativeVideoProcessing(task.status))
  for (const task of running) {
    const apiKey = videoTaskKeyMap[task.id]
    if (!apiKey) continue
    try {
      const status = await getCreativeVideoStatus(apiKey, task.id)
      const next = grokStatusToCreativeStatus(status?.status)
      if (next) task.status = next
      if (status?.model) task.model = status.model
      if (status?.video?.duration) task.duration_seconds = Number(status.video.duration)
    } catch {
      // Keep the local task row; the next poll can recover.
    }
  }
  if (running.length > 0) {
    await loadVideoTasks()
    manageVideoPolling()
  }
}

function manageVideoPolling() {
  const hasProcessingTask = videoTasks.value.some(task => isCreativeVideoProcessing(task.status))
  const shouldPoll = activeTab.value === 'video' && hasProcessingTask
  if (shouldPoll && !videoPollTimer) {
    videoPollTimer = setInterval(() => {
      void refreshRunningVideos()
    }, 8000)
  } else if (!shouldPoll && videoPollTimer) {
    clearInterval(videoPollTimer)
    videoPollTimer = null
  }

  if (hasProcessingTask && !videoElapsedTimer) {
    videoNow.value = Date.now()
    videoElapsedTimer = setInterval(() => {
      videoNow.value = Date.now()
    }, 30 * 1000)
  } else if (!hasProcessingTask && videoElapsedTimer) {
    clearInterval(videoElapsedTimer)
    videoElapsedTimer = null
  }
}

function stopVideoPolling() {
  if (videoPollTimer) {
    clearInterval(videoPollTimer)
    videoPollTimer = null
  }
  if (videoElapsedTimer) {
    clearInterval(videoElapsedTimer)
    videoElapsedTimer = null
  }
}

function grokStatusToCreativeStatus(status: string) {
  const normalized = String(status || '').toLowerCase()
  if (normalized === 'done') return 'completed'
  if (['pending', 'running', 'queued', 'processing'].includes(normalized)) return 'running'
  if (normalized === 'failed' || normalized === 'error') return 'failed'
  if (normalized === 'expired') return 'expired'
  return ''
}

function creativeVideoStatusLabel(status: string) {
  const labels: Record<string, string> = {
    queued: '排队中',
    submitted: '已提交',
    pending: '排队中',
    processing: '生成中',
    running: '生成中',
    completed: '已完成',
    done: '已完成',
    succeeded: '已完成',
    success: '已完成',
    failed: '失败',
    expired: '已过期',
    output_deleted: '已清理',
  }
  const normalized = String(status || '').toLowerCase()
  return labels[normalized] || status || '-'
}

function isCreativeVideoCompleted(status: string) {
  return ['completed', 'done', 'succeeded', 'success'].includes(String(status || '').toLowerCase())
}

function isCreativeVideoProcessing(status: string) {
  return ['queued', 'submitted', 'pending', 'running', 'processing'].includes(String(status || '').toLowerCase())
}

function isCreativeVideoTerminal(status: string) {
  const normalized = String(status || '').toLowerCase()
  return isCreativeVideoCompleted(normalized) || ['failed', 'expired', 'output_deleted'].includes(normalized)
}

function creativeVideoStartedAt(task: CreativeVideoTask) {
  return Number(task.submitted_at || task.created_at || 0)
}

function creativeVideoElapsedSeconds(task: CreativeVideoTask) {
  const startedAt = creativeVideoStartedAt(task)
  if (!startedAt || !isCreativeVideoProcessing(task.status)) return 0
  return Math.max(0, Math.floor(videoNow.value / 1000) - startedAt)
}

function creativeVideoElapsedText(task: CreativeVideoTask) {
  const seconds = creativeVideoElapsedSeconds(task)
  if (!seconds) return ''
  if (seconds < 60) return '已等待不到 1 分钟'
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `已等待 ${minutes} 分钟`
  const hours = Math.floor(minutes / 60)
  const restMinutes = minutes % 60
  return restMinutes ? `已等待 ${hours} 小时 ${restMinutes} 分钟` : `已等待 ${hours} 小时`
}

function creativeVideoProgressHint(task: CreativeVideoTask) {
  const seconds = creativeVideoElapsedSeconds(task)
  if (!seconds) return ''
  const minutes = Math.floor(seconds / 60)
  if (minutes >= 15) return '这个视频已经等待较久，可能是上游排队或任务处理偏慢。页面会继续自动刷新，也可以稍后回来查看。'
  if (minutes >= 5) return '视频生成通常需要几分钟，复杂画面或高峰期会更久一些。任务还在自动刷新，完成后会显示预览和下载。'
  return '任务已提交，正在排队或生成中。你可以先处理别的内容，完成后会出现在这里。'
}

function creativeVideoProgressHintClass(task: CreativeVideoTask) {
  return creativeVideoElapsedSeconds(task) >= 15 * 60
    ? 'bg-amber-50 text-amber-700 dark:bg-amber-900/20 dark:text-amber-300'
    : 'bg-primary-50 text-primary-700 dark:bg-primary-900/20 dark:text-primary-300'
}

function creativeVideoPillClass(status: string) {
  const normalized = String(status || '').toLowerCase()
  if (isCreativeVideoCompleted(normalized)) return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-200'
  if (normalized === 'failed' || normalized === 'expired' || normalized === 'output_deleted') return 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-200'
  return 'bg-teal-100 text-teal-700 dark:bg-teal-900/30 dark:text-teal-200'
}

function shortDateTime(timestamp?: number) {
  if (!timestamp) return '-'
  const date = new Date(timestamp * 1000)
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  const hour = String(date.getHours()).padStart(2, '0')
  const minute = String(date.getMinutes()).padStart(2, '0')
  return `${month}/${day} ${hour}:${minute}`
}

function videoExpiresAt(task: CreativeVideoTask) {
  if (task.output_expires_at) return Number(task.output_expires_at)
  const createdAt = Number(task.created_at || 0)
  if (!createdAt) return 0
  return createdAt + videoLimits.retentionDays * 24 * 60 * 60
}

function formatVideoFileSize(bytes?: number | null) {
  if (!bytes || bytes <= 0) return '-'
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`
  return `${(bytes / 1024 / 1024).toFixed(2)} MB`
}

function formatImageFileSize(bytes?: number | null) {
  if (!bytes || bytes <= 0) return '-'
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`
  return `${(bytes / 1024 / 1024).toFixed(2)} MB`
}

function imageExtensionFromMime(mime?: string | null) {
  const normalized = String(mime || '').toLowerCase()
  if (normalized.includes('png')) return 'png'
  if (normalized.includes('webp')) return 'webp'
  if (normalized.includes('svg')) return 'svg'
  if (normalized.includes('jpeg') || normalized.includes('jpg')) return 'jpg'
  return ''
}

function previewImageFilename(item: BatchImageDetailItem, blob?: Blob | null) {
  const extension = String(item.file_extension || imageExtensionFromMime(blob?.type || item.mime_type) || 'png')
    .replace(/^\./, '')
    .toLowerCase()
  const base = String(item.custom_id || 'image')
    .replace(/[^\w.-]+/g, '_')
    .replace(/^_+|_+$/g, '') || 'image'
  return `${base}.${extension}`
}

function formatVideoCost(cost?: number | null) {
  return cost == null ? '-' : `$${Number(cost).toFixed(2)}`
}

function formatVideoElapsed(task: CreativeVideoTask) {
  if (task.elapsed_seconds != null) {
    const seconds = Number(task.elapsed_seconds)
    if (seconds < 60) return `${seconds} 秒`
    const minutes = Math.floor(seconds / 60)
    const rest = seconds % 60
    return rest ? `${minutes} 分 ${rest} 秒` : `${minutes} 分钟`
  }
  return isCreativeVideoProcessing(task.status) ? creativeVideoElapsedText(task) || '-' : '-'
}

function videoExpiresAtText(task: CreativeVideoTask) {
  const expiresAt = videoExpiresAt(task)
  return expiresAt ? formatDate(expiresAt) : '-'
}

function videoRemainingDays(task: CreativeVideoTask) {
  const expiresAt = videoExpiresAt(task)
  if (!expiresAt) return 0
  return Math.max(0, Math.ceil((expiresAt - Math.floor(Date.now() / 1000)) / (24 * 60 * 60)))
}

function videoRetentionLabel(task: CreativeVideoTask) {
  if (task.status === 'output_deleted') return '已清理'
  if (task.status === 'expired') return '已过期'
  const days = videoRemainingDays(task)
  return days > 0 ? `${days}天` : '将过期'
}

function videoExpiryHint(task: CreativeVideoTask) {
  return `到期后将自动删除，预计 ${videoExpiresAtText(task)} 到期；超过保留条数时可能提前清理。`
}

function reuseVideoTask(task: CreativeVideoTask) {
  activeTab.value = 'video'
  creativePrompt.value = task.prompt_preview || ''
  videoForm.prompt = task.prompt_preview || ''
  videoForm.model = task.model || videoForm.model
  videoForm.resolution = task.resolution || videoForm.resolution
  videoForm.duration = task.duration_seconds || videoForm.duration
  closeVideoPreview()
}

function copyVideoPrompt(task: CreativeVideoTask) {
  void copyToClipboard(task.prompt_preview || '', '提示词已复制')
}

function scheduleVideoThumbnails() {
  const tasks = videoTasks.value
    .filter(task => isCreativeVideoCompleted(task.status) && !videoThumbnailUrls[task.id] && !videoThumbnailLoadingIds.has(task.id) && !videoThumbnailFailedIds.has(task.id))
    .slice(0, 12)
  for (const task of tasks) {
    void ensureVideoThumbnail(task)
  }
}

async function ensureVideoThumbnail(task: CreativeVideoTask) {
  const apiKey = videoTaskKeyMap[task.id]
  if (!apiKey || videoThumbnailUrls[task.id] || videoThumbnailLoadingIds.has(task.id)) return
  videoThumbnailLoadingIds.add(task.id)
  try {
    const blob = await downloadCreativeVideo(apiKey, task.id)
    task.file_size_bytes = blob.size
    task.content_type = blob.type || task.content_type
    videoThumbnailUrls[task.id] = await createVideoThumbnail(blob)
  } catch {
    videoThumbnailFailedIds.add(task.id)
  } finally {
    videoThumbnailLoadingIds.delete(task.id)
  }
}

function waitForVideoEvent(video: HTMLVideoElement, event: string, timeout = 8000) {
  return new Promise<void>((resolve, reject) => {
    const timer = window.setTimeout(() => {
      cleanup()
      reject(new Error(`Timed out waiting for ${event}`))
    }, timeout)
    const cleanup = () => {
      window.clearTimeout(timer)
      video.removeEventListener(event, onEvent)
      video.removeEventListener('error', onError)
    }
    const onEvent = () => {
      cleanup()
      resolve()
    }
    const onError = () => {
      cleanup()
      reject(new Error('Video failed to load'))
    }
    video.addEventListener(event, onEvent, { once: true })
    video.addEventListener('error', onError, { once: true })
  })
}

async function createVideoThumbnail(blob: Blob) {
  const url = URL.createObjectURL(blob)
  const video = document.createElement('video')
  video.muted = true
  video.playsInline = true
  video.preload = 'metadata'
  video.src = url
  try {
    video.load()
    await waitForVideoEvent(video, 'loadedmetadata')
    const targetTime = Math.min(0.2, Math.max(0, (video.duration || 1) / 10))
    if (Number.isFinite(targetTime) && targetTime > 0) {
      video.currentTime = targetTime
      await waitForVideoEvent(video, 'seeked')
    }
    const width = video.videoWidth || 640
    const height = video.videoHeight || 360
    const canvas = document.createElement('canvas')
    canvas.width = width
    canvas.height = height
    const ctx = canvas.getContext('2d')
    if (!ctx) throw new Error('Canvas is unavailable')
    ctx.drawImage(video, 0, 0, width, height)
    return canvas.toDataURL('image/jpeg', 0.78)
  } finally {
    video.removeAttribute('src')
    video.load()
    URL.revokeObjectURL(url)
  }
}

async function downloadVideoTask(task: CreativeVideoTask) {
  const apiKey = videoTaskKeyMap[task.id]
  if (!apiKey) {
    appStore.showError('未找到该任务对应的 API Key')
    return
  }
  videoDownloadingId.value = task.id
  try {
    const blob = await downloadCreativeVideo(apiKey, task.id)
    task.file_size_bytes = blob.size
    task.content_type = blob.type || task.content_type
    saveBlob(blob, `${task.id}.mp4`)
    await loadVideoTasks()
  } catch (error: any) {
    appStore.showError(error?.message || '下载视频失败')
  } finally {
    videoDownloadingId.value = ''
  }
}

async function openVideoDetail(task: CreativeVideoTask) {
  videoDetailTask.value = task
  if (isCreativeVideoCompleted(task.status)) {
    await previewVideoTask(task)
  }
}

async function previewVideoTask(task: CreativeVideoTask) {
  const apiKey = videoTaskKeyMap[task.id]
  if (!apiKey) {
    appStore.showError('未找到该任务对应的 API Key')
    return
  }
  videoPreviewingId.value = task.id
  videoPreviewError.value = ''
  try {
    if (videoPreviewUrl.value) {
      URL.revokeObjectURL(videoPreviewUrl.value)
      videoPreviewUrl.value = ''
    }
    videoDetailTask.value = task
    const blob = await downloadCreativeVideo(apiKey, task.id)
    task.file_size_bytes = blob.size
    task.content_type = blob.type || task.content_type
    videoPreviewUrl.value = URL.createObjectURL(blob)
    videoPreviewTitle.value = task.prompt_preview || task.model
    if (!videoThumbnailUrls[task.id]) {
      try {
        videoThumbnailUrls[task.id] = await createVideoThumbnail(blob)
      } catch {
        // Preview can still play even if thumbnail capture fails.
      }
    }
  } catch (error: any) {
    videoPreviewError.value = '视频加载失败，可以先下载到本地播放。'
    appStore.showError(error?.message || '加载视频预览失败')
  } finally {
    videoPreviewingId.value = ''
  }
}

function handleVideoPreviewError() {
  videoPreviewError.value = '当前浏览器无法预览这个视频，可能是视频编码不被支持。你可以先下载播放。'
}

function closeVideoPreview() {
  if (videoPreviewUrl.value) {
    URL.revokeObjectURL(videoPreviewUrl.value)
  }
  videoPreviewUrl.value = ''
  videoPreviewTitle.value = '视频预览'
  videoPreviewError.value = ''
  videoDetailTask.value = null
}

async function removeVideoTask(task: CreativeVideoTask) {
  if (videoDeletingId.value) return
  if (!window.confirm('确认删除这条视频任务记录吗？删除后将不再显示，视频仍受上游有效期限制。')) return
  videoDeletingId.value = task.id
  const apiKey = videoTaskKeyMap[task.id]
  if (!apiKey) {
    videoTasks.value = videoTasks.value.filter(row => row.id !== task.id)
    delete videoThumbnailUrls[task.id]
    if (videoDetailTask.value?.id === task.id) closeVideoPreview()
    videoDeletingId.value = ''
    return
  }
  try {
    await deleteCreativeVideoTask(apiKey, task.id)
    videoTasks.value = videoTasks.value.filter(row => row.id !== task.id)
    delete videoTaskKeyMap[task.id]
    delete videoThumbnailUrls[task.id]
    if (videoDetailTask.value?.id === task.id) closeVideoPreview()
  } catch (error: any) {
    appStore.showError(error?.message || '删除视频任务失败')
  } finally {
    videoDeletingId.value = ''
  }
}

function applyFilters() {
  pagination.page = 1
  selectedJobIds.value = new Set()
  void loadBatchJobs()
}

function resetFilters() {
  filters.taskName = ''
  filters.apiKeyId = ''
  filters.status = ''
  filters.downloaded = ''
  applyFilters()
}

function listOptions(): BatchImageJobsListOptions {
  const options: BatchImageJobsListOptions = {
    limit: pagination.page_size,
    cursor: String((pagination.page - 1) * pagination.page_size),
  }
  if (filters.taskName.trim()) options.taskName = filters.taskName.trim()
  if (filters.status) options.status = filters.status
  if (filters.downloaded) options.downloaded = filters.downloaded
  return options
}

function toJobRow(job: BatchImageJob, key = selectedApiKey.value): BatchImageJobRow {
  return {
    id: job.id,
    task_name: job.task_name || defaultTaskName(job.created_at),
    parent_batch_id: job.parent_batch_id || null,
    status: job.status,
    model: job.model,
    provider: job.provider,
    item_count: job.item_count,
    success_count: job.success_count,
    fail_count: job.fail_count,
    estimated_cost: job.estimated_cost,
    hold_amount: job.hold_amount,
    actual_cost: job.actual_cost,
    created_at: job.created_at,
    downloaded_at: job.downloaded_at,
    api_key_id: key?.id || 0,
    api_key_name: key?.name || '',
    child_count: 0,
  }
}

function applyChildCounts(rows: BatchImageJobRow[]) {
  const counts = new Map<string, number>()
  for (const row of rows) {
    if (!row.parent_batch_id) continue
    counts.set(row.parent_batch_id, (counts.get(row.parent_batch_id) || 0) + 1)
  }
  return rows.map(row => ({ ...row, child_count: counts.get(row.id) || 0 }))
}

function displayJob<T extends Pick<BatchImageJob, 'id' | 'parent_batch_id' | 'status' | 'item_count' | 'success_count' | 'fail_count' | 'estimated_cost' | 'hold_amount' | 'actual_cost'>>(job: T): T {
  if (job.parent_batch_id) return job
  const children = childrenByParent.value.get(job.id) || []
  if (!children.length) return job

  const childSuccess = children.reduce((sum, child) => sum + child.success_count, 0)
  const childEstimated = children.reduce((sum, child) => sum + child.estimated_cost, 0)
  const childHold = children.reduce((sum, child) => sum + child.hold_amount, 0)
  const childActual = children.reduce((sum, child) => sum + (child.actual_cost || 0), 0)
  const childActualReady = children.every(child => child.actual_cost !== null)
  const successCount = Math.min(job.item_count, job.success_count + childSuccess)
  const failCount = Math.max(0, job.item_count - successCount)
  const actualCost = job.actual_cost === null
    ? (childActualReady ? childActual : null)
    : job.actual_cost + childActual

  return {
    ...job,
    success_count: successCount,
    fail_count: failCount,
    status: failCount === 0 && TERMINAL_STATUSES.has(job.status) ? 'completed' : job.status,
    estimated_cost: job.estimated_cost + childEstimated,
    hold_amount: job.hold_amount + childHold,
    actual_cost: actualCost,
  }
}

function hasChildJobs(batchId: string) {
  return (childrenByParent.value.get(batchId) || []).length > 0
}

function toggleChildRows(batchId: string) {
  const next = new Set(expandedParentIds.value)
  if (next.has(batchId)) next.delete(batchId)
  else next.add(batchId)
  expandedParentIds.value = next
}

function closeMoreMenu() {
  openMoreJobId.value = ''
}

function toggleMoreMenu(job: BatchImageJobRow, event: MouseEvent) {
  if (openMoreJobId.value === job.id) {
    closeMoreMenu()
    return
  }
  const trigger = event.currentTarget as HTMLElement | null
  const rect = trigger?.getBoundingClientRect()
  if (!rect) return
  const menuWidth = 176
  const margin = 8
  const left = Math.max(margin, Math.min(rect.right - menuWidth, window.innerWidth - menuWidth - margin))
  const top = Math.min(rect.bottom + margin, window.innerHeight - 96)
  moreMenuStyle.value = {
    left: `${left}px`,
    top: `${Math.max(margin, top)}px`,
  }
  openMoreJobId.value = job.id
}

function cancelPromptPopoverClose() {
  if (!promptPopoverCloseTimer) return
  clearTimeout(promptPopoverCloseTimer)
  promptPopoverCloseTimer = null
}

function cancelPromptPopoverOpen() {
  if (!promptPopoverOpenTimer) return
  clearTimeout(promptPopoverOpenTimer)
  promptPopoverOpenTimer = null
}

function closePromptPopover() {
  cancelPromptPopoverOpen()
  cancelPromptPopoverClose()
  promptPopover.visible = false
  promptPopover.text = ''
  promptPopover.style = {}
  activePromptPopoverTarget = null
}

function schedulePromptPopoverClose() {
  cancelPromptPopoverOpen()
  cancelPromptPopoverClose()
  promptPopoverCloseTimer = setTimeout(() => {
    closePromptPopover()
  }, 180)
}

function schedulePromptPopoverOpen(event: MouseEvent | PointerEvent, text: string) {
  const target = event.currentTarget as HTMLElement | null
  if (!target) return
  const value = String(text || '').trim()
  if (!value || value === '-') return
  activePromptPopoverTarget = target
  cancelPromptPopoverOpen()
  cancelPromptPopoverClose()
  promptPopoverOpenTimer = setTimeout(() => {
    if (activePromptPopoverTarget !== target || !document.body.contains(target)) return
    openPromptPopover(target, value)
  }, 520)
}

function showPromptPopover(event: MouseEvent | FocusEvent, text: string) {
  const value = String(text || '').trim()
  if (!value || value === '-') return
  const target = event.currentTarget as HTMLElement | null
  cancelPromptPopoverClose()
  cancelPromptPopoverOpen()
  if (!target) return
  activePromptPopoverTarget = target
  openPromptPopover(target, value)
}

function openPromptPopover(target: HTMLElement, value: string) {
  const rect = target.getBoundingClientRect()
  if (!rect) return
  const viewportWidth = window.innerWidth || 1280
  const viewportHeight = window.innerHeight || 720
  const width = Math.min(440, Math.max(320, viewportWidth - 32))
  const left = Math.max(16, Math.min(rect.left, viewportWidth - width - 16))
  const estimatedHeight = 178
  const preferredTop = rect.bottom + 8
  const top = preferredTop + estimatedHeight > viewportHeight
    ? Math.max(16, rect.top - estimatedHeight - 8)
    : preferredTop
  promptPopover.text = value
  promptPopover.style = {
    left: `${left}px`,
    top: `${top}px`,
    width: `${width}px`,
  }
  promptPopover.visible = true
}

function copyPromptPopover() {
  if (!promptPopover.text) return
  void copyToClipboard(promptPopover.text, t('batchImage.promptPopover.copied'))
}

async function loadBatchJobs() {
  clearImageJobPreviews()
  const previewRequestID = ++imagePreviewRequestSeq
  const keys = filteredApiKeys.value
  if (!keys.length) {
    batchJobs.value = []
    pagination.has_more = false
    return
  }
  loadingJobs.value = true
  closeMoreMenu()
  try {
    const options = listOptions()
    const results = await Promise.all(keys.map(async (key) => {
      const result = await listBatchImageJobs(key.key, options)
      return {
        hasMore: Boolean(result.has_more),
        rows: (result.data || []).map(job => toJobRow(job, key)),
      }
    }))
    batchJobs.value = applyChildCounts(results
      .flatMap(result => result.rows)
      .sort((a, b) => b.created_at - a.created_at)
      .slice(0, pagination.page_size))
    pagination.has_more = results.some(result => result.hasMore)
    selectedJobIds.value = new Set([...selectedJobIds.value].filter(id => visibleBatchJobs.value.some(job => job.id === id)))
    if (previewRequestID === imagePreviewRequestSeq) {
      void hydrateRecentImageJobPreviews(batchJobs.value)
    }
  } catch (error: any) {
    appStore.showError(batchImageErrorMessage(error, batchImageText('loadJobsFailed')))
  } finally {
    loadingJobs.value = false
  }
}

function upsertJob(job: BatchImageJob) {
  const next = toJobRow(job)
  const index = batchJobs.value.findIndex(item => item.id === job.id)
  if (index >= 0) {
    const rows = [...batchJobs.value]
    rows[index] = { ...next, is_child: rows[index].is_child }
    batchJobs.value = applyChildCounts(rows)
    return
  }
  batchJobs.value = applyChildCounts([next, ...batchJobs.value].slice(0, pagination.page_size))
}

function handlePageChange(page: number) {
  if (page < 1 || page === pagination.page) return
  pagination.page = page
  selectedJobIds.value = new Set()
  void loadBatchJobs()
}

function handlePageSizeChange(value: string | number | boolean | null) {
  if (value === null || typeof value === 'boolean') return
  const nextSize = Math.min(Math.max(Number(value) || 20, 1), 100)
  pagination.page_size = nextSize
  pagination.page = 1
  setPersistedPageSize(nextSize)
  selectedJobIds.value = new Set()
  void loadBatchJobs()
}

function openCreateModal() {
  showCreateModal.value = true
  if (!apiKeys.value.length) {
    void loadApiKeys()
  }
}

function closeCreateModal() {
  if (submitting.value) return
  showCreateModal.value = false
  resetCreateDraft()
}

function resetCreateDraft() {
  form.taskName = ''
  form.responseMimeType = 'image/png'
  promptRows.value = []
  promptDraft.value = ''
  customIdDraft.value = ''
  outputCountDraft.value = 1
  referenceImageDrafts.value = []
}

function closeDetail() {
  closePromptPopover()
  currentJob.value = null
  selectedBatchId.value = ''
  selectedBatchApiKeyId.value = 0
  items.value = []
  clearItemPreviews()
}

function keyForSelectedBatch(): ApiKey | null {
  if (selectedBatchApiKeyId.value) {
    const key = geminiApiKeys.value.find(item => item.id === selectedBatchApiKeyId.value)
    if (key) return key
  }
  return selectedApiKey.value
}

function requireApiKey(): ApiKey | null {
  if (!selectedApiKey.value) {
    appStore.showError(batchImageText('selectApiKey'))
    return null
  }
  return selectedApiKey.value
}

function validateForm(): boolean {
  if (!requireApiKey()) return false
  if (!form.model) {
    appStore.showError(availableBatchImageModels.value.length === 0 ? batchImageText('noModelsForKey') : batchImageText('selectModel'))
    return false
  }
  if (parsedItems.value.length === 0) {
    appStore.showError(batchImageText('promptRequired'))
    return false
  }
  if (estimatedOutputCount.value > BATCH_IMAGE_MAX_OUTPUTS_PER_JOB) {
    appStore.showError(batchImageText('tooManyOutputImages'))
    return false
  }
  const refLimit = selectedModelReferenceLimit.value
  if (promptRows.value.some(row => row.reference_images.length > refLimit)) {
    appStore.showError(batchImageText('tooManyReferenceImages'))
    return false
  }
  return true
}

async function submitJob(): Promise<boolean> {
  if (submitting.value) return false
  if (promptDraft.value.trim()) addPromptRow()
  if (!validateForm()) return false
  const key = requireApiKey()
  if (!key) return false
	  submitting.value = true
	  try {
	    const job = await submitBatchImageJob(
	      key.key,
	      {
	        model: form.model,
        task_name: form.taskName.trim() || defaultTaskName(),
        image_size: composerSize.value,
        aspect_ratio: composerAspectRatio.value === '跟随原图' ? undefined : composerAspectRatio.value,
        response_mime_type: form.responseMimeType,
        items: parsedItems.value,
	      },
	      `sub2api-ui-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`,
	    )
	    currentJob.value = job
	    selectedBatchId.value = job.id
	    selectedBatchApiKeyId.value = key.id
	    items.value = []
	    upsertJob(job)
	    showCreateModal.value = false
	    resetCreateDraft()
	    appStore.showSuccess(batchImageText('submitted'))
	    void loadItems()
	    startPolling()
    return true
  } catch (error: any) {
    appStore.showError(batchImageErrorMessage(error, batchImageText('submitFailed')))
    return false
  } finally {
    submitting.value = false
  }
}

async function refreshSelected() {
  if (!selectedBatchId.value) return
  const key = keyForSelectedBatch() || requireApiKey()
  if (!key) return
  refreshing.value = true
  try {
    const job = await getBatchImageJob(key.key, selectedBatchId.value)
    currentJob.value = job
    upsertJob(job)
    if (TERMINAL_STATUSES.has(job.status)) {
      stopPolling()
      void hydrateRecentImageJobPreviews(batchJobs.value)
    }
  } catch (error: any) {
    appStore.showError(batchImageErrorMessage(error, batchImageText('refreshFailed')))
  } finally {
    refreshing.value = false
  }
}

async function refreshDetail() {
  await Promise.all([
    refreshSelected(),
    loadItems(),
  ])
}

function selectJob(batchId: string) {
  const row = batchJobs.value.find(job => job.id === batchId)
  if (row?.api_key_id && geminiApiKeys.value.some(key => key.id === row.api_key_id)) {
    form.apiKeyId = row.api_key_id
    selectedBatchApiKeyId.value = row.api_key_id
  } else {
    selectedBatchApiKeyId.value = 0
  }
  selectedBatchId.value = batchId
  currentJob.value = null
  items.value = []
  void refreshSelected()
  void loadItems()
}

function startPolling() {
  stopPolling()
  pollTimer = setInterval(() => {
    if (!currentJob.value || TERMINAL_STATUSES.has(currentJob.value.status)) {
      stopPolling()
      return
    }
    void refreshSelected()
  }, 8000)
}

function stopPolling() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

function canCancel(job: Pick<BatchImageJob, 'status'>) {
  return !TERMINAL_STATUSES.has(job.status)
}

function canDownload(job: Pick<BatchImageJob, 'status' | 'success_count'>) {
  return job.status === 'completed' && job.success_count > 0
}

function canRetry(job: Pick<BatchImageJob, 'status' | 'fail_count'>) {
  const display = 'id' in job ? displayJob(job as BatchImageJob) : job
  return TERMINAL_STATUSES.has(display.status) && display.fail_count > 0
}

function isDownloadingJob(batchId: string) {
  return downloading.value && downloadingBatchId.value === batchId
}

function applyJobApiKey(job: BatchImageJobRow | Pick<BatchImageJob, 'id'>) {
  if ('api_key_id' in job && job.api_key_id && geminiApiKeys.value.some(key => key.id === job.api_key_id)) {
    form.apiKeyId = job.api_key_id
  }
}

function apiKeyForJob(job: BatchImageJobRow | Pick<BatchImageJob, 'id'>): ApiKey | null {
  if ('api_key_id' in job && job.api_key_id) {
    return geminiApiKeys.value.find(key => key.id === job.api_key_id) || null
  }
  return selectedApiKey.value
}

function toggleJobSelection(batchId: string, checked: boolean) {
  const next = new Set(selectedJobIds.value)
  if (checked) next.add(batchId)
  else next.delete(batchId)
  selectedJobIds.value = next
}

function toggleAllVisible(checked: boolean) {
  const next = new Set(selectedJobIds.value)
  for (const job of visibleBatchJobs.value) {
    if (checked) next.add(job.id)
    else next.delete(job.id)
  }
  selectedJobIds.value = next
}

function canDeleteRecord(job: Pick<BatchImageJob, 'status'>) {
  return TERMINAL_STATUSES.has(job.status)
}

async function cancelSelected() {
  if (!currentJob.value) return
  const key = keyForSelectedBatch() || requireApiKey()
  if (!key) return
  if (!window.confirm(batchImageText('cancelConfirm'))) return
  cancelling.value = true
  try {
    const job = await cancelBatchImageJob(key.key, currentJob.value.id)
    currentJob.value = job
    upsertJob(job)
    appStore.showSuccess(batchImageText('cancelled'))
  } catch (error: any) {
    appStore.showError(batchImageErrorMessage(error, batchImageText('cancelFailed')))
  } finally {
    cancelling.value = false
  }
}

async function downloadSelected() {
  if (!currentJob.value) return
  await downloadJob(currentJob.value)
}

async function retrySelected() {
  if (!currentJob.value) return
  await retryFailedJob(currentJob.value)
}

async function retryFailedJob(job: BatchImageJobRow | BatchImageJob) {
  if (!canRetry(job) || retryingBatchId.value) return
  closeMoreMenu()
  const key = apiKeyForJob(job) || keyForSelectedBatch() || requireApiKey()
  if (!key) return
  retryingBatchId.value = job.id
  try {
    const sourceItems = await ensureItemsForRetry(key.key, job.id)
    const failedItems = sourceItems
      .filter(item => item.status === 'failed')
      .map(item => ({ custom_id: retryCustomID(item.custom_id), prompt: String(item.prompt_preview || '').trim() }))
      .filter(item => item.prompt)
    if (failedItems.length === 0) {
      appStore.showError(batchImageText('retryMissingPrompts'))
      return
    }
    const retryJob = await submitBatchImageJob(
      key.key,
      {
        model: job.model,
        task_name: `${job.task_name || defaultTaskName()} ${t('batchImage.messages.retryTaskNameSuffix')}`,
        parent_batch_id: rootBatchIdForRetry(job),
        provider: job.provider,
        image_size: '1K',
        response_mime_type: form.responseMimeType,
        items: failedItems,
      },
      `sub2api-ui-retry-${job.id}-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`,
    )
    currentJob.value = retryJob
    selectedBatchId.value = retryJob.id
    selectedBatchApiKeyId.value = key.id
    items.value = []
    upsertJob(retryJob)
    if (retryJob.parent_batch_id) {
      expandedParentIds.value = new Set([...expandedParentIds.value, retryJob.parent_batch_id])
    }
    appStore.showSuccess(batchImageText('retrySubmitted'))
    void loadItems()
    startPolling()
  } catch (error: any) {
    appStore.showError(batchImageErrorMessage(error, batchImageText('retryFailed')))
  } finally {
    retryingBatchId.value = ''
  }
}

async function ensureItemsForRetry(apiKey: string, batchId: string) {
  if (selectedBatchId.value === batchId && items.value.length > 0) {
    return items.value
  }
  const result = await listBatchImageItems(apiKey, batchId)
  return result.data || []
}

function retryCustomID(customID: string) {
  const base = String(customID || 'item').replace(/[^\w.-]+/g, '_').replace(/^_+|_+$/g, '') || 'item'
  return `${base}_retry_${Date.now().toString(36)}`
}

function rootBatchIdForRetry(job: BatchImageJobRow | BatchImageJob) {
  return job.parent_batch_id || job.id
}

async function downloadJob(job: (BatchImageJobRow | Pick<BatchImageJob, 'id'>)) {
  if (downloading.value) return
  closeMoreMenu()
  applyJobApiKey(job)
  const key = apiKeyForJob(job) || requireApiKey()
  if (!key) return
  downloading.value = true
  downloadingBatchId.value = job.id
  try {
    const blob = await downloadBatchImageZip(key.key, job.id)
    saveBlob(blob, `${job.id}.zip`)
    markJobDownloaded(job.id)
  } catch (error: any) {
    appStore.showError(batchImageErrorMessage(error, batchImageText('downloadFailed')))
  } finally {
    downloading.value = false
    downloadingBatchId.value = ''
  }
}

async function downloadSelectedJobs() {
  if (bulkDownloading.value || selectedDownloadableRows.value.length === 0) return
  bulkDownloading.value = true
  try {
    for (const row of selectedDownloadableRows.value) {
      const key = apiKeyForJob(row)
      if (!key) continue
      downloading.value = true
      downloadingBatchId.value = row.id
      const blob = await downloadBatchImageZip(key.key, row.id)
      saveBlob(blob, `${row.id}.zip`)
      markJobDownloaded(row.id)
    }
    appStore.showSuccess(batchImageText('batchDownloadStarted'))
  } catch (error: any) {
    appStore.showError(batchImageErrorMessage(error, batchImageText('downloadFailed')))
  } finally {
    bulkDownloading.value = false
    downloading.value = false
    downloadingBatchId.value = ''
  }
}

async function deleteJob(job: BatchImageJobRow) {
  if (!canDeleteRecord(job) || deletingBatchId.value) return
  closeMoreMenu()
  const key = apiKeyForJob(job)
  if (!key) return
  if (!window.confirm(batchImageText('deleteConfirm'))) return
  deletingBatchId.value = job.id
  try {
    await deleteBatchImageJobRecord(key.key, job.id)
    removeJobFromList(job.id)
    appStore.showSuccess(batchImageText('deleted'))
  } catch (error: any) {
    appStore.showError(batchImageErrorMessage(error, batchImageText('deleteFailed')))
  } finally {
    deletingBatchId.value = ''
  }
}

async function deleteSelectedJobs() {
  const rows = selectedRows.value.filter(job => canDeleteRecord(job))
  if (bulkDeleting.value || rows.length === 0) return
  if (!window.confirm(batchImageText('deleteSelectedConfirm'))) return
  bulkDeleting.value = true
  try {
    for (const row of rows) {
      const key = apiKeyForJob(row)
      if (!key) continue
      deletingBatchId.value = row.id
      await deleteBatchImageJobRecord(key.key, row.id)
      removeJobFromList(row.id)
    }
    appStore.showSuccess(batchImageText('deleted'))
  } catch (error: any) {
    appStore.showError(batchImageErrorMessage(error, batchImageText('deleteFailed')))
  } finally {
    bulkDeleting.value = false
    deletingBatchId.value = ''
  }
}

function markJobDownloaded(batchId: string) {
  const downloadedAt = Math.floor(Date.now() / 1000)
  batchJobs.value = batchJobs.value.map(job => job.id === batchId ? { ...job, downloaded_at: job.downloaded_at || downloadedAt } : job)
  if (currentJob.value?.id === batchId && !currentJob.value.downloaded_at) {
    currentJob.value = { ...currentJob.value, downloaded_at: downloadedAt }
  }
}

function removeJobFromList(batchId: string) {
  batchJobs.value = batchJobs.value.filter(job => job.id !== batchId)
  toggleJobSelection(batchId, false)
  if (currentJob.value?.id === batchId) closeDetail()
}

function canLoadItemPreview(item: BatchImageItem) {
  return (item.status === 'succeeded' || item.status === 'success') && item.image_count > 0
}

function isSuccessfulImageItem(item: Pick<BatchImageItem, 'status' | 'image_count'>) {
  return (item.status === 'succeeded' || item.status === 'success') && item.image_count > 0
}

function detailRootBatchId() {
  return currentJob.value?.parent_batch_id || selectedBatchId.value || currentJob.value?.id || ''
}

function isChildDetailItem(item: Pick<BatchImageDetailItem, 'batch_id'>) {
  const rootBatchId = detailRootBatchId()
  return Boolean(rootBatchId && item.batch_id && item.batch_id !== rootBatchId)
}

function retrySourceCustomID(customID: string) {
  return String(customID || '').replace(/(?:_retry_[a-z0-9]+)+$/i, '')
}

function isRecoveredOriginalFailure(item: BatchImageDetailItem) {
  const rootBatchId = detailRootBatchId()
  return Boolean(
    rootBatchId
    && item.batch_id === rootBatchId
    && item.status === 'failed'
    && recoveredOriginalCustomIds.value.has(item.custom_id),
  )
}

function detailItemRowClass(item: BatchImageDetailItem) {
  if (isRecoveredOriginalFailure(item)) {
    return 'bg-gray-50/80 text-gray-400 hover:bg-gray-100/80 dark:bg-dark-900/60 dark:text-gray-500 dark:hover:bg-dark-800/70'
  }
  return 'hover:bg-gray-50/70 dark:hover:bg-dark-800/60'
}

function previewCacheSupported() {
  return typeof window !== 'undefined' && 'indexedDB' in window
}

function previewCacheKey(batchId: string, customID: string, imageIndex = 0) {
  return [batchId, customID, imageIndex].map(part => encodeURIComponent(String(part))).join(':')
}

function itemPreviewKey(item: Pick<BatchImageItem, 'batch_id' | 'custom_id'>) {
  return previewCacheKey(item.batch_id || selectedBatchId.value || currentJob.value?.id || '', item.custom_id, 0)
}

function idbRequest<T>(request: IDBRequest<T>): Promise<T> {
  return new Promise((resolve, reject) => {
    request.onsuccess = () => resolve(request.result)
    request.onerror = () => reject(request.error)
  })
}

function openPreviewCacheDB(): Promise<IDBDatabase | null> {
  if (!previewCacheSupported()) return Promise.resolve(null)
  if (previewCacheDBPromise) return previewCacheDBPromise

  previewCacheDBPromise = new Promise((resolve) => {
    const request = window.indexedDB.open(PREVIEW_CACHE_DB_NAME, 1)
    request.onupgradeneeded = () => {
      const db = request.result
      if (!db.objectStoreNames.contains(PREVIEW_CACHE_STORE_NAME)) {
        const store = db.createObjectStore(PREVIEW_CACHE_STORE_NAME, { keyPath: 'key' })
        store.createIndex('lastAccessedAt', 'lastAccessedAt', { unique: false })
      }
    }
    request.onsuccess = () => resolve(request.result)
    request.onerror = () => resolve(null)
    request.onblocked = () => resolve(null)
  })
  return previewCacheDBPromise
}

async function getCachedPreviewBlob(cacheKey: string): Promise<Blob | null> {
  const db = await openPreviewCacheDB()
  if (!db) return null
  const record = await idbRequest<PreviewCacheRecord | undefined>(
    db.transaction(PREVIEW_CACHE_STORE_NAME, 'readonly').objectStore(PREVIEW_CACHE_STORE_NAME).get(cacheKey),
  ).catch(() => undefined)
  if (!record?.blob) return null

  const now = Date.now()
  if (now - record.createdAt > PREVIEW_CACHE_MAX_AGE_MS) {
    void deleteCachedPreview(cacheKey)
    return null
  }
  void touchCachedPreview(cacheKey, now)
  return record.blob
}

async function hydrateCachedItemPreviews(detailItems: BatchImageDetailItem[]) {
  const previewableItems = detailItems.filter(item => canLoadItemPreview(item))
  if (!previewableItems.length || !previewCacheSupported()) return

  await Promise.all(previewableItems.map(async (item) => {
    const batchId = item.batch_id || selectedBatchId.value || currentJob.value?.id || ''
    const previewKey = itemPreviewKey(item)
    if (!batchId || itemPreviewUrls[previewKey] || previewErrorIds.value.has(previewKey)) return
    const cached = await getCachedPreviewBlob(previewCacheKey(batchId, item.custom_id, 0)).catch(() => null)
    if (!cached || itemPreviewUrls[previewKey]) return
    itemPreviewUrls[previewKey] = URL.createObjectURL(cached)
  }))
}

async function hydrateRecentImageJobPreviews(jobs: BatchImageJobRow[]) {
  const requestID = imagePreviewRequestSeq
  const recentJobs = jobs
    .filter(job => !job.parent_batch_id)
    .slice(0, IMAGE_JOB_PREVIEW_LIMIT)
  if (!recentJobs.length) return

  await Promise.all(recentJobs.map(async (job) => {
    if (requestID !== imagePreviewRequestSeq || imageJobPreviewUrls[job.id]) return
    const key = apiKeyForJob(job)
    if (!key) return
    imageJobPreviewLoadingIds.value = new Set([...imageJobPreviewLoadingIds.value, job.id])
    try {
      const previewJobs = detailJobsForBatch(job.id)
      const candidates: BatchImageDetailItem[] = []
      for (const previewJob of previewJobs) {
        try {
          const result = await listBatchImageItems(key.key, previewJob.id)
          candidates.push(...(result.data || []).map(item => ({
            ...item,
            batch_id: previewJob.id,
            source_task_name: previewJob.task_name || previewJob.id,
          })))
        } catch {
          // A preview is optional; keep the task card available when details expire.
        }
        if (candidates.some(item => canLoadItemPreview(item))) break
      }
      const item = candidates.find(candidate => canLoadItemPreview(candidate))
      if (!item || requestID !== imagePreviewRequestSeq) return
      const batchID = item.batch_id || job.id
      const cacheKey = previewCacheKey(batchID, item.custom_id, 0)
      try {
        const cached = await getCachedPreviewBlob(cacheKey)
        if (cached) {
          if (requestID === imagePreviewRequestSeq) imageJobPreviewUrls[job.id] = URL.createObjectURL(cached)
          return
        }
        const blob = await getBatchImageItemContent(key.key, batchID, item.custom_id, 0)
        const thumbnail = await createThumbnailBlob(blob).catch(() => blob)
        if (requestID !== imagePreviewRequestSeq) return
        imageJobPreviewUrls[job.id] = URL.createObjectURL(thumbnail)
        if (thumbnail !== blob || thumbnail.size <= 1024 * 1024) {
          void putCachedPreviewBlob(cacheKey, thumbnail)
        }
      } catch {
        // A preview is optional; keep the placeholder when the content is unavailable.
      }
    } finally {
      const next = new Set(imageJobPreviewLoadingIds.value)
      next.delete(job.id)
      imageJobPreviewLoadingIds.value = next
    }
  }))
}

async function putCachedPreviewBlob(cacheKey: string, blob: Blob) {
  const db = await openPreviewCacheDB()
  if (!db) return
  const now = Date.now()
  const record: PreviewCacheRecord = {
    key: cacheKey,
    blob,
    size: blob.size,
    createdAt: now,
    lastAccessedAt: now,
  }
  await idbRequest(db.transaction(PREVIEW_CACHE_STORE_NAME, 'readwrite').objectStore(PREVIEW_CACHE_STORE_NAME).put(record)).catch(() => null)
  void cleanupPreviewCache()
}

async function touchCachedPreview(cacheKey: string, lastAccessedAt: number) {
  const db = await openPreviewCacheDB()
  if (!db) return
  const record = await idbRequest<PreviewCacheRecord | undefined>(
    db.transaction(PREVIEW_CACHE_STORE_NAME, 'readonly').objectStore(PREVIEW_CACHE_STORE_NAME).get(cacheKey),
  ).catch(() => undefined)
  if (!record) return
  record.lastAccessedAt = lastAccessedAt
  await idbRequest(db.transaction(PREVIEW_CACHE_STORE_NAME, 'readwrite').objectStore(PREVIEW_CACHE_STORE_NAME).put(record)).catch(() => null)
}

async function deleteCachedPreview(cacheKey: string) {
  const db = await openPreviewCacheDB()
  if (!db) return
  await idbRequest(db.transaction(PREVIEW_CACHE_STORE_NAME, 'readwrite').objectStore(PREVIEW_CACHE_STORE_NAME).delete(cacheKey)).catch(() => null)
}

async function cleanupPreviewCache() {
  const db = await openPreviewCacheDB()
  if (!db) return
  const records = await idbRequest<PreviewCacheRecord[]>(
    db.transaction(PREVIEW_CACHE_STORE_NAME, 'readonly').objectStore(PREVIEW_CACHE_STORE_NAME).getAll(),
  ).catch(() => [])
  if (!records.length) return

  const now = Date.now()
  const sorted = [...records].sort((a, b) => a.lastAccessedAt - b.lastAccessedAt)
  const deleteKeys = new Set<string>()
  let totalBytes = 0
  let keptCount = 0

  for (const record of sorted) {
    if (now - record.createdAt > PREVIEW_CACHE_MAX_AGE_MS) {
      deleteKeys.add(record.key)
      continue
    }
    totalBytes += record.size || record.blob?.size || 0
    keptCount += 1
  }

  for (const record of sorted) {
    if (deleteKeys.has(record.key)) continue
    if (keptCount <= PREVIEW_CACHE_MAX_ENTRIES && totalBytes <= PREVIEW_CACHE_MAX_BYTES) break
    deleteKeys.add(record.key)
    totalBytes -= record.size || record.blob?.size || 0
    keptCount -= 1
  }

  if (!deleteKeys.size) return
  const store = db.transaction(PREVIEW_CACHE_STORE_NAME, 'readwrite').objectStore(PREVIEW_CACHE_STORE_NAME)
  for (const key of deleteKeys) {
    store.delete(key)
  }
}

async function createThumbnailBlob(blob: Blob): Promise<Blob> {
  const source = await loadPreviewImageSource(blob)
  const width = source.width
  const height = source.height
  const scale = Math.min(1, PREVIEW_THUMBNAIL_MAX_EDGE / Math.max(width, height))
  const targetWidth = Math.max(1, Math.round(width * scale))
  const targetHeight = Math.max(1, Math.round(height * scale))
  const canvas = document.createElement('canvas')
  canvas.width = targetWidth
  canvas.height = targetHeight
  const ctx = canvas.getContext('2d')
  if (!ctx) throw new Error('canvas unavailable')
  ctx.drawImage(source.image, 0, 0, targetWidth, targetHeight)
  source.close()
  return await new Promise<Blob>((resolve, reject) => {
    canvas.toBlob((thumbnail) => {
      if (thumbnail) resolve(thumbnail)
      else reject(new Error('thumbnail unavailable'))
    }, 'image/webp', PREVIEW_THUMBNAIL_QUALITY)
  })
}

async function loadPreviewImageSource(blob: Blob): Promise<{ image: PreviewImageSource, width: number, height: number, close: () => void }> {
  if ('createImageBitmap' in window) {
    const bitmap = await window.createImageBitmap(blob)
    return {
      image: bitmap,
      width: bitmap.width,
      height: bitmap.height,
      close: () => bitmap.close(),
    }
  }

  const url = URL.createObjectURL(blob)
  try {
    const image = await new Promise<HTMLImageElement>((resolve, reject) => {
      const img = new Image()
      img.onload = () => resolve(img)
      img.onerror = () => reject(new Error('image unavailable'))
      img.src = url
    })
    return {
      image,
      width: image.naturalWidth || image.width,
      height: image.naturalHeight || image.height,
      close: () => URL.revokeObjectURL(url),
    }
  } catch (error) {
    URL.revokeObjectURL(url)
    throw error
  }
}

async function loadItems() {
  const batchId = selectedBatchId.value || currentJob.value?.id || ''
  if (!batchId) return
  const key = keyForSelectedBatch() || requireApiKey()
  if (!key) return
  loadingItems.value = true
  try {
    clearItemPreviews()
    const jobs = detailJobsForBatch(batchId)
    const results = await Promise.all(jobs.map(async (job) => {
      const result = await listBatchImageItems(key.key, job.id)
      return (result.data || []).map(item => ({
        ...item,
        batch_id: job.id,
        source_task_name: detailSourceName(job, batchId),
      }))
    }))
    const detailItems = results.flat()
    items.value = detailItems
    void hydrateCachedItemPreviews(detailItems)
  } catch (error: any) {
    appStore.showError(batchImageErrorMessage(error, batchImageText('loadItemsFailed')))
  } finally {
    loadingItems.value = false
  }
}

function detailJobsForBatch(batchId: string): BatchImageJobRow[] {
  const row = batchJobs.value.find(job => job.id === batchId)
  const base = row || (currentJob.value && currentJob.value.id === batchId ? toJobRow(currentJob.value, keyForSelectedBatch() || selectedApiKey.value) : null)
  if (!base) return []
  if (base.parent_batch_id) return [base]
  return [base, ...(childrenByParent.value.get(base.id) || [])]
}

function detailSourceName(job: Pick<BatchImageJobRow, 'id' | 'task_name' | 'parent_batch_id'>, rootBatchId: string) {
  const name = job.task_name || job.id
  if (job.id === rootBatchId) return t('batchImage.detail.mainTask', { name })
  return t('batchImage.detail.childTask', { name })
}

async function loadItemPreview(item: BatchImageItem) {
  const batchId = item.batch_id || selectedBatchId.value || currentJob.value?.id || ''
  const previewKey = itemPreviewKey(item)
  if (!batchId || !canLoadItemPreview(item) || (itemPreviewUrls[previewKey] && !previewErrorIds.value.has(previewKey))) return
  const key = keyForSelectedBatch() || requireApiKey()
  if (!key) return
  const cacheKey = previewCacheKey(batchId, item.custom_id, 0)
  previewLoadingIds.value = new Set([...previewLoadingIds.value, previewKey])
  try {
    previewErrorIds.value = new Set([...previewErrorIds.value].filter(id => id !== previewKey))
    if (itemPreviewUrls[previewKey]) {
      URL.revokeObjectURL(itemPreviewUrls[previewKey])
      delete itemPreviewUrls[previewKey]
    }
    const cached = await getCachedPreviewBlob(cacheKey)
    if (cached) {
      itemPreviewUrls[previewKey] = URL.createObjectURL(cached)
      return
    }
    const blob = await getBatchImageItemContent(key.key, batchId, item.custom_id, 0)
    const thumbnail = await createThumbnailBlob(blob).catch(() => blob)
    itemPreviewUrls[previewKey] = URL.createObjectURL(thumbnail)
    if (thumbnail !== blob || thumbnail.size <= 1024 * 1024) {
      void putCachedPreviewBlob(cacheKey, thumbnail)
    }
  } catch (error: any) {
    previewErrorIds.value = new Set([...previewErrorIds.value, previewKey])
    appStore.showError(batchImageErrorMessage(error, batchImageText('loadPreviewFailed')))
  } finally {
    const next = new Set(previewLoadingIds.value)
    next.delete(previewKey)
    previewLoadingIds.value = next
  }
}

async function openJobPreview(job: BatchImageJobRow) {
  const key = apiKeyForJob(job)
  if (!key) return
  closePromptPopover()
  selectedBatchId.value = job.id
  selectedBatchApiKeyId.value = key.id
  form.apiKeyId = key.id
  currentJob.value = null
  items.value = []
  clearItemPreviews()

  try {
    const [loadedJob] = await Promise.all([
      getBatchImageJob(key.key, job.id),
      (async () => {
        const detailJobs = detailJobsForBatch(job.id)
        const results = await Promise.all(detailJobs.map(async (detailJob) => {
          const result = await listBatchImageItems(key.key, detailJob.id)
          return (result.data || []).map(item => ({
            ...item,
            batch_id: detailJob.id,
            source_task_name: detailSourceName(detailJob, job.id),
          }))
        }))
        items.value = results.flat()
        void hydrateCachedItemPreviews(items.value)
      })(),
    ])
    currentJob.value = loadedJob
    upsertJob(loadedJob)
    const firstPreview = previewImageCandidates.value[0]
    if (firstPreview) {
      await loadItemPreview(firstPreview)
      openImagePreview(firstPreview)
    } else {
      selectJob(job.id)
    }
  } catch (error: any) {
    appStore.showError(batchImageErrorMessage(error, batchImageText('loadItemsFailed')))
    selectJob(job.id)
  }
}

async function selectPreviewImage(item: BatchImageDetailItem) {
  previewImageItem.value = item
  await loadItemPreview(item)
  if (previewImageItem.value === item) {
    await loadPreviewImageOriginal(item)
  }
}

function openImagePreview(item: BatchImageDetailItem) {
  previewImageItem.value = item
  void loadItemPreview(item)
  void loadPreviewImageOriginal(item)
}

function closeImagePreview() {
  previewImageFullRequestSeq += 1
  previewImageItem.value = null
  previewImageFullLoading.value = false
  previewImageFullError.value = ''
  previewImageDownloading.value = false
  previewImageFullBlob.value = null
  if (previewImageFullUrl.value) {
    URL.revokeObjectURL(previewImageFullUrl.value)
    previewImageFullUrl.value = ''
  }
}

function previewImageApiKey(item: BatchImageDetailItem | null = previewImageItem.value): ApiKey | null {
  if (!item) return null
  const batchId = item.batch_id || selectedBatchId.value || currentJob.value?.id || ''
  const row = batchJobs.value.find(job => job.id === batchId)
  if (row) return apiKeyForJob(row)
  return keyForSelectedBatch() || selectedApiKey.value
}

async function loadPreviewImageOriginal(item: BatchImageDetailItem | null = previewImageItem.value) {
  if (!item) return
  const batchId = item.batch_id || selectedBatchId.value || currentJob.value?.id || ''
  const key = previewImageApiKey(item)
  if (!batchId || !key) return
  const requestSeq = ++previewImageFullRequestSeq
  previewImageFullLoading.value = true
  previewImageFullError.value = ''
  if (previewImageFullUrl.value) {
    URL.revokeObjectURL(previewImageFullUrl.value)
    previewImageFullUrl.value = ''
  }
  previewImageFullBlob.value = null
  try {
    const blob = await getBatchImageItemContent(key.key, batchId, item.custom_id, 0)
    if (requestSeq !== previewImageFullRequestSeq || previewImageItem.value !== item) return
    previewImageFullBlob.value = blob
    previewImageFullUrl.value = URL.createObjectURL(blob)
  } catch (error: any) {
    if (requestSeq !== previewImageFullRequestSeq) return
    previewImageFullError.value = batchImageErrorMessage(error, batchImageText('loadPreviewFailed'))
  } finally {
    if (requestSeq === previewImageFullRequestSeq) {
      previewImageFullLoading.value = false
    }
  }
}

function reloadPreviewImageOriginal() {
  void loadPreviewImageOriginal()
}

function handleImagePreviewDisplayError() {
  previewImageFullError.value = t('batchImage.imagePreview.previewFailed')
}

function copyPreviewImagePrompt() {
  const prompt = String(previewImageItem.value?.prompt_preview || '').trim()
  if (!prompt) return
  void copyToClipboard(prompt, t('batchImage.promptPopover.copied'))
}

async function downloadPreviewImageOriginal() {
  const item = previewImageItem.value
  if (!item || previewImageDownloading.value) return
  const batchId = item.batch_id || selectedBatchId.value || currentJob.value?.id || ''
  const key = previewImageApiKey(item)
  if (!batchId || !key) return
  previewImageDownloading.value = true
  try {
    const blob = previewImageFullBlob.value || await getBatchImageItemContent(key.key, batchId, item.custom_id, 0)
    previewImageFullBlob.value = blob
    saveBlob(blob, previewImageFilename(item, blob))
  } catch (error: any) {
    appStore.showError(batchImageErrorMessage(error, batchImageText('downloadFailed')))
  } finally {
    previewImageDownloading.value = false
  }
}

async function downloadPreviewImageJob() {
  const item = previewImageItem.value
  if (!item) return
  const batchId = item.batch_id || selectedBatchId.value || currentJob.value?.id || ''
  if (!batchId) return
  await downloadJob(batchJobs.value.find(job => job.id === batchId) || { id: batchId })
}

function handlePreviewError(customID: string) {
  if (itemPreviewUrls[customID]) {
    URL.revokeObjectURL(itemPreviewUrls[customID])
    delete itemPreviewUrls[customID]
  }
  previewErrorIds.value = new Set([...previewErrorIds.value, customID])
}

function handleJobPreviewError(jobID: string) {
  if (imageJobPreviewUrls[jobID]) {
    URL.revokeObjectURL(imageJobPreviewUrls[jobID])
    delete imageJobPreviewUrls[jobID]
  }
}

function clearImageJobPreviews() {
  imagePreviewRequestSeq += 1
  for (const url of Object.values(imageJobPreviewUrls)) {
    if (url) URL.revokeObjectURL(url)
  }
  for (const key of Object.keys(imageJobPreviewUrls)) {
    delete imageJobPreviewUrls[key]
  }
  imageJobPreviewLoadingIds.value = new Set()
}

function clearItemPreviews() {
  closePromptPopover()
  for (const url of Object.values(itemPreviewUrls)) {
    if (url) URL.revokeObjectURL(url)
  }
  for (const key of Object.keys(itemPreviewUrls)) {
    delete itemPreviewUrls[key]
  }
  previewLoadingIds.value = new Set()
  previewErrorIds.value = new Set()
  previewImageItem.value = null
}

function copyInstruction() {
  void copyToClipboard(agentInstruction.value, batchImageText('copiedInstruction'))
}

function copyVideoApiDocs() {
  void copyToClipboard(videoApiDocsText.value, '视频 API 文档已复制')
}

function statusLabel(jobOrStatus: BatchImageStatus | Pick<BatchImageJob, 'status' | 'success_count' | 'fail_count'>) {
  const status = typeof jobOrStatus === 'string' ? jobOrStatus : jobOrStatus.status
  if (typeof jobOrStatus !== 'string' && status === 'completed' && jobOrStatus.fail_count > 0) {
    if (jobOrStatus.success_count > 0) return t('batchImage.status.partialSuccess')
    return t('batchImage.status.allFailed')
  }
  const statusKeys: Record<string, string> = {
    queued: 'queued',
    running: 'running',
    indexing: 'processingResults',
    processing_results: 'processingResults',
    settling: 'settling',
    completed: 'completed',
    failed: 'failed',
    cancelled: 'cancelled',
    output_deleted: 'outputDeleted',
  }
  const key = statusKeys[status]
  return key ? t(`batchImage.status.${key}`) : status
}

function statusBadgeClass(jobOrStatus: BatchImageStatus | Pick<BatchImageJob, 'status' | 'success_count' | 'fail_count'>) {
  const status = typeof jobOrStatus === 'string' ? jobOrStatus : jobOrStatus.status
  if (typeof jobOrStatus !== 'string' && status === 'completed' && jobOrStatus.fail_count > 0) {
    if (jobOrStatus.success_count > 0) return 'badge-warning'
    return 'badge-danger'
  }
  if (status === 'completed') return 'badge-success'
  if (status === 'failed' || status === 'cancelled') return 'badge-danger'
  if (status === 'output_deleted') return 'badge-gray'
  return 'badge-primary'
}

function itemStatusLabel(status: string) {
  const statusKeys: Record<string, string> = {
    pending: 'pending',
    succeeded: 'succeeded',
    success: 'succeeded',
    failed: 'failed',
    cancelled: 'cancelled',
  }
  const key = statusKeys[status]
  return key ? t(`batchImage.itemStatus.${key}`) : status
}

function itemDisplayStatusLabel(item: BatchImageDetailItem) {
  if (isRecoveredOriginalFailure(item)) return t('batchImage.itemStatus.recovered')
  return itemStatusLabel(item.status)
}

function itemStatusBadgeClass(status: string) {
  if (status === 'succeeded' || status === 'success') return 'badge-success'
  if (status === 'failed' || status === 'cancelled') return 'badge-danger'
  return 'badge-primary'
}

function itemDisplayStatusBadgeClass(item: BatchImageDetailItem) {
  if (isRecoveredOriginalFailure(item)) return 'badge-gray'
  return itemStatusBadgeClass(item.status)
}

function itemResultLabel(item: BatchImageDetailItem) {
  if (isRecoveredOriginalFailure(item)) return t('batchImage.itemResult.recoveredByRetry')
  if (item.error) return friendlyItemError(item.error)
  if (item.status === 'succeeded' || item.status === 'success') {
    return itemPreviewUrls[itemPreviewKey(item)] ? t('batchImage.itemResult.readyPreview') : t('batchImage.itemResult.readyDownload')
  }
  if (item.status === 'failed') return t('batchImage.itemResult.noUsableImage')
  if (item.status === 'cancelled') return t('batchImage.itemResult.cancelled')
  return t('batchImage.itemResult.waiting')
}

function itemResultClass(item: BatchImageDetailItem) {
  if (isRecoveredOriginalFailure(item)) return 'bg-gray-100 text-gray-500 ring-gray-200 dark:bg-dark-800 dark:text-gray-400 dark:ring-dark-700'
  if (item.error || item.status === 'failed' || item.status === 'cancelled') return 'bg-red-50 text-red-700 ring-red-100 dark:bg-red-950/30 dark:text-red-300 dark:ring-red-900/50'
  if (item.status === 'succeeded' || item.status === 'success') return 'bg-emerald-50 text-emerald-700 ring-emerald-100 dark:bg-emerald-950/30 dark:text-emerald-300 dark:ring-emerald-900/50'
  return 'bg-gray-50 text-gray-500 ring-gray-200 dark:bg-dark-800 dark:text-gray-400 dark:ring-dark-700'
}

function friendlyItemError(error: BatchImageItem['error']) {
  if (!error) return '-'
  if (error.code === 'EMPTY_IMAGE_OUTPUT') return t('batchImage.itemResult.emptyImageOutput')
  if (error.code === 'PROVIDER_ITEM_FAILED') return t('batchImage.itemResult.providerItemFailed')
  return error.message || error.code || '-'
}

function formatMoney(value: number | null | undefined) {
  if (value === null || value === undefined || Number.isNaN(Number(value))) return '$0.00'
  return `$${Number(value).toFixed(2)}`
}

function terminalZeroCost(job: Pick<BatchImageJob, 'status' | 'actual_cost'>) {
  return job.actual_cost === null && (job.status === 'failed' || job.status === 'cancelled')
}

function costLabel(job: Pick<BatchImageJob, 'status' | 'hold_amount' | 'actual_cost'>) {
  if (job.actual_cost !== null) return formatMoney(job.actual_cost)
  if (terminalZeroCost(job)) return formatMoney(0)
  return t('batchImage.detail.holdCost', { amount: formatMoney(job.hold_amount) })
}

type BatchImageTextKey =
  | 'loadKeysFailed'
  | 'loadModelsFailed'
  | 'loadJobsFailed'
  | 'selectApiKey'
  | 'noModelsForKey'
  | 'selectModel'
  | 'promptRequired'
  | 'submitted'
  | 'submitFailed'
  | 'refreshFailed'
  | 'cancelConfirm'
  | 'cancelled'
  | 'cancelFailed'
  | 'batchDownloadStarted'
	  | 'downloadFailed'
	  | 'retrySubmitted'
	  | 'retryFailed'
	  | 'retryMissingPrompts'
  | 'deleteConfirm'
  | 'deleteSelectedConfirm'
  | 'deleted'
  | 'deleteFailed'
	  | 'loadItemsFailed'
	  | 'loadPreviewFailed'
  | 'copiedInstruction'
  | 'loadingModels'
  | 'noModels'
  | 'noModelsHint'
  | 'noCompatibleAccount'
  | 'unsupportedProvider'
  | 'providerSubmitFailed'
  | 'vertexGcsBucketMissing'
  | 'queueFailed'
  | 'billingHoldFailed'
  | 'groupDisabled'
  | 'pricingMissing'
  | 'insufficientBalance'
  | 'invalidModel'
  | 'invalidItems'
  | 'duplicateCustomId'
  | 'promptTooLong'
  | 'invalidReferenceImage'
  | 'tooManyReferenceImages'
  | 'referenceImagesTooLarge'
  | 'tooManyOutputImages'
  | 'idempotencyConflict'
  | 'notReady'
  | 'outputDeleted'
  | 'resultMissing'
  | 'itemFailed'
  | 'itemImageIndexOutOfRange'
  | 'downloadLimited'
  | 'downloadTooLarge'
  | 'deleteNotReady'
  | 'disabled'
  | 'authRequired'
  | 'adminReference'
  | 'errorReference'

function isZhLocale() {
  return String(locale.value || '').toLowerCase().startsWith('zh')
}

function batchImageText(key: BatchImageTextKey) {
  return t(`batchImage.messages.${key}`)
}

function batchImageErrorReference(error: any) {
  const parts: string[] = []
  const code = String(error?.code || '').trim()
  const requestId = String(error?.requestId || '').trim()
  const status = String(error?.status || '').trim()
  if (code) parts.push(t('batchImage.messages.errorCodeRef', { code }))
  if (requestId) parts.push(t('batchImage.messages.requestIdRef', { id: requestId }))
  if (!code && status) parts.push(t('batchImage.messages.httpStatusRef', { status }))
  return parts.length ? `（${parts.join(isZhLocale() ? '，' : ', ')}）` : ''
}

function batchImageAdminError(base: string, error: any) {
  const reference = batchImageErrorReference(error)
  return `${base}${reference ? ` ${reference}` : ''} ${batchImageText('adminReference')}`
}

function batchImagePlainError(base: string) {
  return base
}

function batchImageErrorMessage(error: any, fallback: string) {
  const code = String(error?.code || '').trim()
  const message = String(error?.message || '').trim()
  if (code === 'API_KEY_REQUIRED' || code === '401') {
    return batchImagePlainError(batchImageText('authRequired'))
  }
  if (code === 'BATCH_IMAGE_NO_ACCOUNT_AVAILABLE' || /no compatible batch image account/i.test(message)) {
    return batchImageAdminError(batchImageText('noCompatibleAccount'), error)
  }
  if (code === 'BATCH_IMAGE_UNSUPPORTED_PROVIDER' || /unsupported batch image provider/i.test(message)) {
    return batchImageAdminError(batchImageText('unsupportedProvider'), error)
  }
  if (code === 'BATCH_IMAGE_VERTEX_GCS_BUCKET_MISSING' || code === 'VERTEX_MANAGED_GCS_BUCKET_MISSING') {
    return batchImageAdminError(batchImageText('vertexGcsBucketMissing'), error)
  }
  if (
    code === 'BATCH_IMAGE_PROVIDER_SUBMIT_FAILED' ||
    code === 'BATCH_IMAGE_PROVIDER_MISSING_API_KEY' ||
    code === 'BATCH_IMAGE_PROVIDER_MISSING_SERVICE_ACCOUNT' ||
    code === 'BATCH_IMAGE_PROVIDER_UNSUPPORTED_ACCOUNT'
  ) {
    return batchImageAdminError(batchImageText('providerSubmitFailed'), error)
  }
  if (code === 'BATCH_IMAGE_QUEUE_FAILED' || code === 'BATCH_IMAGE_QUEUE_NOT_CONFIGURED') {
    return batchImageAdminError(batchImageText('queueFailed'), error)
  }
  if (code === 'BATCH_IMAGE_BILLING_HOLD_FAILED') {
    return batchImageAdminError(batchImageText('billingHoldFailed'), error)
  }
  if (code === 'BATCH_IMAGE_GROUP_DISABLED') {
    return batchImagePlainError(batchImageText('groupDisabled'))
  }
  if (code === 'BATCH_IMAGE_SETTLEMENT_PRICING_MISSING') {
    return batchImageAdminError(batchImageText('pricingMissing'), error)
  }
  if (code === 'BATCH_IMAGE_INSUFFICIENT_BALANCE') {
    return batchImagePlainError(batchImageText('insufficientBalance'))
  }
  if (code === 'BATCH_IMAGE_INVALID_MODEL') {
    return batchImageText('invalidModel')
  }
  if (code === 'BATCH_IMAGE_INVALID_ITEMS') {
    return batchImageText('invalidItems')
  }
  if (code === 'BATCH_IMAGE_DUPLICATE_CUSTOM_ID') {
    return batchImageText('duplicateCustomId')
  }
  if (code === 'BATCH_IMAGE_PROMPT_TOO_LONG') {
    return batchImageText('promptTooLong')
  }
  if (code === 'BATCH_IMAGE_INVALID_REFERENCE_IMAGE') {
    return batchImageText('invalidReferenceImage')
  }
  if (code === 'BATCH_IMAGE_TOO_MANY_REFERENCE_IMAGES') {
    return batchImageText('tooManyReferenceImages')
  }
  if (code === 'BATCH_IMAGE_REFERENCE_IMAGES_TOO_LARGE') {
    return batchImageText('referenceImagesTooLarge')
  }
  if (code === 'BATCH_IMAGE_TOO_MANY_OUTPUT_IMAGES') {
    return batchImageText('tooManyOutputImages')
  }
  if (code === 'BATCH_IMAGE_IDEMPOTENCY_CONFLICT') {
    return batchImagePlainError(batchImageText('idempotencyConflict'))
  }
  if (code === 'BATCH_IMAGE_NOT_READY') {
    return batchImageText('notReady')
  }
  if (code === 'BATCH_IMAGE_OUTPUT_DELETED') {
    return batchImageText('outputDeleted')
  }
  if (code === 'BATCH_IMAGE_RESULT_MISSING') {
    return batchImageAdminError(batchImageText('resultMissing'), error)
  }
  if (code === 'BATCH_IMAGE_ITEM_FAILED') {
    return batchImagePlainError(batchImageText('itemFailed'))
  }
  if (code === 'BATCH_IMAGE_ITEM_IMAGE_INDEX_OUT_OF_RANGE') {
    return batchImagePlainError(batchImageText('itemImageIndexOutOfRange'))
  }
  if (code === 'BATCH_IMAGE_DOWNLOAD_LIMITED') {
    return batchImageText('downloadLimited')
  }
  if (code === 'BATCH_IMAGE_DOWNLOAD_TOO_LARGE') {
    return batchImageText('downloadTooLarge')
  }
  if (code === 'BATCH_IMAGE_RECORD_DELETE_NOT_READY') {
    return batchImagePlainError(batchImageText('deleteNotReady'))
  }
  if (code === 'BATCH_IMAGE_DISABLED') {
    return batchImageAdminError(batchImageText('disabled'), error)
  }
  if (code === 'INTERNAL_ERROR' || code === '500') {
    return batchImageAdminError(fallback, error)
  }
  if (isZhLocale()) {
    const detail = message ? `${batchImageText('errorReference')}：${message}` : batchImageText('adminReference')
    return `${fallback}。${detail} ${batchImageErrorReference(error)}`
  }
  return message || fallback
}

function formatDate(timestamp: number) {
  if (!timestamp) return ''
  return new Date(timestamp * 1000).toLocaleString()
}

function defaultTaskName(timestamp?: number) {
  const date = timestamp ? new Date(timestamp * 1000) : new Date()
  return date.toLocaleString()
}

onMounted(() => {
  void appStore.fetchPublicSettings()
  void refreshPage()
  void cleanupPreviewCache()
  previewCacheCleanupTimer = setInterval(() => {
    void cleanupPreviewCache()
  }, 60 * 60 * 1000)
  document.addEventListener('click', closeMoreMenu)
  window.addEventListener('resize', closeMoreMenu)
  window.addEventListener('scroll', closeMoreMenu, true)
  window.addEventListener('resize', closePromptPopover)
  window.addEventListener('scroll', closePromptPopover, true)
})

watch(
  () => form.apiKeyId,
  () => {
    void loadAvailableModels()
  },
)

watch(
  () => activeTab.value,
  (tab) => {
    if (tab === 'video') {
      void loadVideoTasks()
    } else {
      stopVideoPolling()
    }
  },
)

watch(
  () => imageTool.value,
  (tool) => {
    composerAspectRatio.value = tool === 'edit' ? '跟随原图' : '1:1'
  },
)

watch(
  () => templateDrawerState.value,
  (state) => writeStoredString(STORAGE_TEMPLATE_DRAWER_KEY, state),
)

watch(
  () => templateCategory.value,
  (category) => writeStoredString(STORAGE_TEMPLATE_CATEGORY_KEY, category),
)

watch(
  () => videoTasks.value.map(task => `${task.id}:${task.status}`).join('|'),
  () => manageVideoPolling(),
)

watch(
  () => form.model,
  () => {
    const limit = selectedModelReferenceLimit.value
    if (limit <= 0) {
      referenceImageDrafts.value = []
      return
    }
    if (referenceImageDrafts.value.length > limit) {
      referenceImageDrafts.value = referenceImageDrafts.value.slice(0, limit)
    }
  },
)

onBeforeUnmount(() => {
  stopPolling()
  stopVideoPolling()
  closeVideoPreview()
  if (previewCacheCleanupTimer) {
    clearInterval(previewCacheCleanupTimer)
    previewCacheCleanupTimer = null
  }
  clearImageJobPreviews()
  clearItemPreviews()
  document.removeEventListener('click', closeMoreMenu)
  window.removeEventListener('resize', closeMoreMenu)
  window.removeEventListener('scroll', closeMoreMenu, true)
  window.removeEventListener('resize', closePromptPopover)
  window.removeEventListener('scroll', closePromptPopover, true)
})
</script>

<style scoped>
.vertical-rl {
  writing-mode: vertical-rl;
}

.template-preview-sheen {
  background:
    linear-gradient(120deg, transparent 0%, rgba(255, 255, 255, 0.34) 42%, transparent 58%),
    radial-gradient(circle at 82% 18%, rgba(255, 255, 255, 0.42), transparent 28%);
  mix-blend-mode: soft-light;
}

.template-preview-portrait {
  background:
    radial-gradient(circle at 50% 32%, rgba(255, 255, 255, 0.9) 0 14%, transparent 15%),
    linear-gradient(180deg, transparent 48%, rgba(15, 23, 42, 0.18) 49% 64%, transparent 65%),
    linear-gradient(135deg, #dbeafe 0%, #f8fafc 42%, #d1fae5 100%);
}

.template-preview-id {
  background:
    radial-gradient(circle at 50% 30%, rgba(255, 255, 255, 0.92) 0 15%, transparent 16%),
    linear-gradient(180deg, transparent 50%, rgba(251, 207, 232, 0.7) 51% 66%, transparent 67%),
    linear-gradient(135deg, #f8fafc 0%, #e0f2fe 54%, #fce7f3 100%);
}

.template-preview-magazine {
  background:
    radial-gradient(circle at 62% 34%, rgba(253, 230, 138, 0.36), transparent 26%),
    linear-gradient(160deg, rgba(17, 24, 39, 0.58) 0 36%, transparent 37%),
    linear-gradient(135deg, #111827 0%, #9f1239 48%, #fde68a 100%);
}

.template-preview-travel {
  background:
    radial-gradient(circle at 78% 22%, rgba(254, 240, 138, 0.9) 0 9%, transparent 10%),
    linear-gradient(180deg, #bae6fd 0%, #fef3c7 52%, #fb7185 100%);
}

.template-preview-cyber {
  background:
    radial-gradient(circle at 62% 42%, rgba(236, 72, 153, 0.7) 0 13%, transparent 14%),
    linear-gradient(90deg, transparent 0 44%, rgba(255, 255, 255, 0.2) 45% 46%, transparent 47%),
    linear-gradient(135deg, #0f172a 0%, #1d4ed8 48%, #db2777 100%);
}

.template-preview-product {
  background:
    radial-gradient(ellipse at 50% 54%, rgba(255, 255, 255, 0.95) 0 17%, transparent 18%),
    radial-gradient(ellipse at 50% 74%, rgba(15, 23, 42, 0.12), transparent 25%),
    linear-gradient(135deg, #ecfccb 0%, #f8fafc 45%, #bfdbfe 100%);
}

.template-preview-wallpaper {
  background:
    radial-gradient(circle at 24% 72%, rgba(255, 255, 255, 0.55), transparent 22%),
    linear-gradient(180deg, #bfdbfe 0%, #fecdd3 46%, #fdf2f8 100%);
}

.template-preview-sticker {
  background:
    radial-gradient(circle at 50% 45%, rgba(255, 255, 255, 0.94) 0 20%, transparent 21%),
    radial-gradient(circle at 43% 41%, rgba(15, 23, 42, 0.34) 0 2%, transparent 3%),
    radial-gradient(circle at 57% 41%, rgba(15, 23, 42, 0.34) 0 2%, transparent 3%),
    linear-gradient(135deg, #fde68a 0%, #a7f3d0 48%, #bfdbfe 100%);
}

.template-preview-bg {
  background:
    linear-gradient(90deg, rgba(255, 255, 255, 0.88) 0 38%, transparent 39%),
    radial-gradient(circle at 72% 38%, rgba(34, 197, 94, 0.28), transparent 24%),
    linear-gradient(135deg, #f8fafc 0%, #bbf7d0 48%, #bae6fd 100%);
}

.template-preview-comic {
  background:
    radial-gradient(circle at 48% 36%, rgba(255, 255, 255, 0.9) 0 18%, transparent 19%),
    linear-gradient(135deg, transparent 0 48%, rgba(15, 23, 42, 0.16) 49% 50%, transparent 51%),
    linear-gradient(135deg, #fef3c7 0%, #fbcfe8 45%, #c7d2fe 100%);
}

.template-preview-pet {
  background:
    radial-gradient(circle at 50% 44%, rgba(251, 191, 36, 0.78) 0 18%, transparent 19%),
    radial-gradient(circle at 45% 40%, rgba(120, 53, 15, 0.28) 0 2%, transparent 3%),
    radial-gradient(circle at 55% 40%, rgba(120, 53, 15, 0.28) 0 2%, transparent 3%),
    linear-gradient(135deg, #fefce8 0%, #ccfbf1 48%, #dbeafe 100%);
}

.template-preview-outfit {
  background:
    linear-gradient(90deg, transparent 0 42%, rgba(255, 255, 255, 0.62) 43% 58%, transparent 59%),
    linear-gradient(135deg, #f5f5f4 0%, #ddd6fe 48%, #fecaca 100%);
}
</style>

<style scoped>
.batch-row-action {
  display: flex !important;
  flex-direction: column !important;
  align-items: center !important;
  justify-content: center !important;
  min-width: 42px;
  line-height: 1;
  outline: none;
}

.batch-row-action:focus {
  outline: none;
}

.batch-row-action :deep(svg) {
  margin-right: 0 !important;
}

.batch-prompt-trigger:focus {
  outline: none;
  box-shadow: none;
}

.batch-prompt-popover {
  user-select: text;
}

.batch-prompt-popover p {
  scrollbar-width: thin;
}

.batch-output-count-select {
  height: 36px;
  min-height: 36px;
  padding-top: 0;
  padding-bottom: 0;
  padding-left: 14px;
  padding-right: 34px;
  line-height: 36px;
}
</style>
