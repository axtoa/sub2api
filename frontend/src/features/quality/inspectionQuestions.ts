import type { InspectionQuestion } from './api'

export function validInspectionQuestions(questions: InspectionQuestion[], limit = 8): boolean {
  return questions.length > 0 && questions.length <= limit && questions.every(q =>
    q.name.trim() && [...q.name].length <= 100 && q.prompt.trim() && [...q.prompt].length <= 4000 &&
    [...q.expected].length <= 1000 && ['exact', 'contains', 'manual'].includes(q.match) &&
    (q.match === 'manual' || q.expected.trim())) &&
    new TextEncoder().encode(questions.map(q => q.name + q.prompt + q.expected).join('')).length <= 24000
}

// Execution stores a copy of content; library IDs are only used by the picker.
export function questionSnapshot(q: InspectionQuestion): InspectionQuestion {
  return { name: q.name, prompt: q.prompt, match: q.match, expected: q.expected }
}
