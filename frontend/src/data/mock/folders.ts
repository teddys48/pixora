import { Folder } from '@/types/folder'

export const MOCK_FOLDERS: Folder[] = [
  {
    id: 'folder-nature',
    name: 'Nature & Landscapes',
    description: 'Mountain peaks, tranquil lakes, and misty forest trails.',
    photoCount: 5,
    coverUrl: 'https://images.unsplash.com/photo-1506744038136-46273834b3fb?w=800&auto=format&fit=crop&q=80',
    createdAt: '2026-08-10T09:00:00Z',
    updatedAt: '2026-10-24T16:20:00Z',
  },
  {
    id: 'folder-architecture',
    name: 'Architecture & Urban',
    description: 'Geometric facades, concrete forms, and city skylines.',
    photoCount: 4,
    coverUrl: 'https://images.unsplash.com/photo-1486406146926-c627a92ad1ab?w=800&auto=format&fit=crop&q=80',
    createdAt: '2026-08-15T11:30:00Z',
    updatedAt: '2026-10-18T10:15:00Z',
  },
  {
    id: 'folder-travel',
    name: 'Travel & Roadtrips',
    description: 'Coastlines, desert highways, and quiet coastal towns.',
    photoCount: 4,
    coverUrl: 'https://images.unsplash.com/photo-1469854523086-cc02fe5d8800?w=800&auto=format&fit=crop&q=80',
    createdAt: '2026-09-01T08:00:00Z',
    updatedAt: '2026-10-12T14:45:00Z',
  },
  {
    id: 'folder-portraits',
    name: 'Portraits & People',
    description: 'Candid moments, studio natural lighting, and street portraits.',
    photoCount: 3,
    coverUrl: 'https://images.unsplash.com/photo-1534528741775-53994a69daeb?w=800&auto=format&fit=crop&q=80',
    createdAt: '2026-09-12T14:00:00Z',
    updatedAt: '2026-10-05T19:30:00Z',
  },
]
