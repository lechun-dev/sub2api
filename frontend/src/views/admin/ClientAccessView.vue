<template>
  <AppLayout><div class="mx-auto max-w-6xl p-4 sm:p-6"><ClientAccessManager ref="manager" :key="`${groupId ?? 'global'}:${initialTab}`" :group-id="groupId" :initial-tab="initialTab" /></div></AppLayout>
</template>
<script setup lang="ts">
import { computed, ref } from 'vue'
import { onBeforeRouteLeave, onBeforeRouteUpdate, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import ClientAccessManager from '@/components/admin/ClientAccessManager.vue'
const route = useRoute()
const { t } = useI18n()
const manager = ref<InstanceType<typeof ClientAccessManager> | null>(null)
const groupId = computed(() => route.query.group === undefined ? undefined : typeof route.query.group === 'string' ? route.query.group : '')
const initialTab = computed(() => typeof route.query.tab === 'string' ? route.query.tab : 'policies')
const canLeave = () => !manager.value?.dirty || window.confirm(t('admin.clientAccess.discard'))
onBeforeRouteLeave(canLeave)
// 2026-10-14 coder(lq): 同一路由切换分组或管理入口也要检查草稿，避免 URL 已变而表单仍在编辑旧组。
onBeforeRouteUpdate(canLeave)
</script>
