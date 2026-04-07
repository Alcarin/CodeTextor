<template>
  <div id="app" class="main-container">
    <!-- TODO: add loading spinner -->
    <header>
      <h1>{{ title }}</h1>
      <nav id="main-nav">
        <a href="/about">About</a>
      </nav>
    </header>
    <main>
      <article class="post">
        <p>Content here</p>
      </article>
    </main>
  </div>
</template>


<script>
import { ref, computed } from 'vue';
import axios from 'axios';

export default {
  setup() {
    const title = ref('My App');
    const count = ref(0);

    function increment() {
      count.value++;
    }

    const doubled = computed(() => count.value * 2);

    axios.get('/api/data').then(res => {
      console.log(res.data);
    });

    return { title, count, increment, doubled };
  }
}
</script>

<style scoped>
.main-container {
  max-width: 1200px;
  margin: 0 auto;
}

#app {
  font-family: 'Inter', sans-serif;
}

/* FIXME: dark mode colors */
.post {
  padding: 20px;
  border-radius: 8px;
}
</style>
