<template>
  <form>
    <h1>{{ $t('loans.new') }}</h1>
    <InputText v-model="form.book" />
    <Calendar v-model="form.due" />
    <input v-model="note" />
    <FancyPicker v-model="form.shelf" />
  </form>
</template>

<script setup lang="ts">
import InputText from 'primevue/inputtext';

definePageMeta({ middleware: ['auth', 'audit'] });
const form = reactive({ book: '', due: '', shelf: '' });
const note = ref('');
</script>
