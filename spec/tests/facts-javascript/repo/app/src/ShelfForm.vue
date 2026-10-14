<template>
  <h1>New shelf</h1>
  <form @submit.prevent="save">
    <InputText v-model="shelf.name" />
    <input v-for="tag in tags" :key="tag" v-model="shelf.tag" />
    <p v-if="shelf.name">Saving {{ shelf.name }} &amp; more</p>
  </form>
</template>

<script setup lang="ts">
import InputText from 'primevue/inputtext';
import { reactive } from 'vue';

const shelf = reactive({ name: '', tag: '' });
const tags = ['a', 'b'];
function save() {}
</script>
