<template>
  <h1>New shelf</h1>
  <InputText v-model="shelf.name" />
  <InputText v-model="shelf.room" />
</template>

<script setup lang="ts">
import InputText from 'primevue/inputtext';
import { reactive } from 'vue';

const shelf = reactive({ name: '', room: '' });
</script>
