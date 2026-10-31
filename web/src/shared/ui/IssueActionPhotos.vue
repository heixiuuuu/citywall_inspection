<template>
  <div v-loading="loading" class="action-photos">
    <div v-if="photos.length" class="photos">
      <div v-for="photo in photos" :key="photo.id">
        <el-image v-if="photo.url" :src="photo.url" :preview-src-list="previewUrls" :initial-index="previewUrls.indexOf(photo.url)" preview-teleported fit="cover" :alt="`处置照片 #${photo.id}`" />
        <span v-else>照片 #{{ photo.id }} 加载失败</span>
      </div>
    </div>
    <el-upload v-if="canUpload" accept="image/jpeg,image/png,image/webp" :show-file-list="false" :disabled="uploading" :before-upload="beforeUpload" :http-request="upload">
      <el-button size="small" :loading="uploading">补充处置照片</el-button>
    </el-upload>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { ElMessage, type UploadRawFile, type UploadRequestOptions } from 'element-plus'
import { uploadIssueActionImage, type IssueActionImage } from '@/shared/api/issue'
import { loadImageBlob } from '@/shared/api/observation'

const props = defineProps<{ issueId: number; actionId: number; images: IssueActionImage[]; canUpload: boolean }>()
const emit = defineEmits<{ uploaded: []; uploading: [pending: boolean] }>()
const photos = ref<{ id: number; url: string }[]>([])
const loading = ref(false)
const uploading = ref(false)
const previewUrls = computed(() => photos.value.map(photo => photo.url).filter(Boolean))
let generation = 0
function release() { photos.value.forEach(photo => { if (photo.url) URL.revokeObjectURL(photo.url) }); photos.value = [] }
watch(() => props.images, async images => {
  const current = ++generation
  release()
  loading.value = true
  const loaded = await Promise.all((images ?? []).map(async image => ({ id: image.id, url: await loadImageBlob(image.id).catch(() => '') })))
  if (current !== generation) { loaded.forEach(photo => { if (photo.url) URL.revokeObjectURL(photo.url) }); return }
  photos.value = loaded
  loading.value = false
}, { immediate: true })
onBeforeUnmount(() => { generation++; release() })

function beforeUpload(file: UploadRawFile) {
  if (file.size > 10 * 1024 * 1024) { ElMessage.warning('照片不得超过 10 MB'); return false }
  return true
}
async function upload(options: UploadRequestOptions) {
  uploading.value = true
  emit('uploading', true)
  try {
    const result = await uploadIssueActionImage(props.issueId, props.actionId, options.file)
    ElMessage.success('处置照片已保存')
    emit('uploaded')
    return result
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '照片上传失败')
    throw error
  } finally { uploading.value = false; emit('uploading', false) }
}
</script>

<style scoped>
.action-photos { margin-top: 8px; }
.photos { display: flex; flex-wrap: wrap; gap: 8px; margin-bottom: 8px; }
.el-image { width: 112px; height: 84px; border-radius: 6px; }
</style>
