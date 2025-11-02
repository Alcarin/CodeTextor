# CodeTextor Frontend

Vue 3 + TypeScript frontend for CodeTextor, a local-first code context provider.

## Features

- **Indexing View**: Select and index project directories
- **Search View**: Semantic code search with similarity scoring
- **Outline View**: Hierarchical file structure visualization
- **Stats View**: Project statistics and metadata

## Project Structure

```
frontend/
├── src/
│   ├── components/      # Reusable Vue components
│   ├── views/           # Main view components
│   │   ├── IndexingView.vue
│   │   ├── SearchView.vue
│   │   ├── OutlineView.vue
│   │   └── StatsView.vue
│   ├── composables/     # Vue composables
│   │   └── useNavigation.ts
│   ├── services/        # Backend service layer
│   │   └── mockBackend.ts
│   ├── types/           # TypeScript type definitions
│   │   └── index.ts
│   ├── App.vue          # Root component
│   ├── main.js          # Application entry point
│   └── env.d.ts         # TypeScript declarations
├── wailsjs/             # Wails-generated bindings
├── package.json
├── tsconfig.json
├── vite.config.js
└── index.html
```

## Setup

### Prerequisites

- Node.js >= 16
- npm or yarn

### Installation

```bash
cd frontend
npm install
```

### Development

Run the development server:

```bash
npm run dev
```

This will start Vite dev server. The application will be available at the URL shown in the terminal.

**Note**: When running standalone (without Wails), the app uses mock backend services. To use the real backend, run the entire application through Wails:

```bash
cd ..
wails dev
```

### Building

Build for production:

```bash
npm run build
```

Type-check without building:

```bash
npm run type-check
```

## Current State

### Implemented

✅ Complete UI structure with 4 main views
✅ Navigation system
✅ Mock backend service layer
✅ TypeScript type definitions
✅ Responsive dark theme UI

### TODO

⏳ Connect to real Wails backend (replace mock services)
⏳ Install npm dependencies
⏳ Add syntax highlighting for code display
⏳ Implement advanced search filters
⏳ Add keyboard shortcuts
⏳ Improve error handling

See [docs/TODO.md](../docs/TODO.md) for the complete task list.

## Mock Backend

The `mockBackend.ts` service provides simulated responses for all backend operations:

- `startIndexing()` - Simulates project indexing with progress
- `semanticSearch()` - Returns mock search results
- `getOutline()` - Generates mock file structure
- `getProjectStats()` - Returns simulated statistics

**To integrate with real backend:**

1. Import Wails bindings: `import { StartIndexing } from '../wailsjs/go/main/App'`
2. Replace mock calls with real bindings in each view
3. Handle async responses and errors appropriately

## Technologies

- **Vue 3** - Progressive JavaScript framework
- **TypeScript** - Type-safe JavaScript
- **Vite** - Next-generation frontend tooling
- **Wails** - Go + Web framework for desktop apps

## Code Style

- All code and comments in English
- Every function must have JSDoc comments
- Use Composition API with `<script setup>`
- Follow DEV_GUIDE.md conventions

## Contributing

Please read [docs/DEV_GUIDE.md](../docs/DEV_GUIDE.md) for development guidelines and coding conventions.

## Recommended IDE Setup

- [VS Code](https://code.visualstudio.com/) + [Volar](https://marketplace.visualstudio.com/items?itemName=Vue.volar)
