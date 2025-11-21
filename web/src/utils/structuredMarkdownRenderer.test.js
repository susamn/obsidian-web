import { describe, it, expect } from 'vitest';
import {
  renderStructuredMarkdown,
  processTags,
  buildOutline,
  formatStats,
  formatReadingTime,
  processBacklinks
} from './structuredMarkdownRenderer';

describe('structuredMarkdownRenderer', () => {
  describe('renderStructuredMarkdown', () => {
    it('should render basic markdown', () => {
      const markdown = '# Hello World\n\nThis is a test.';
      const html = renderStructuredMarkdown(markdown, [], []);

      expect(html).toContain('<h1>Hello World</h1>');
      expect(html).toContain('<p>This is a test.</p>');
    });

    it('should handle empty content', () => {
      const html = renderStructuredMarkdown('', [], []);
      expect(html).toBe('');
    });

    it('should replace wikilinks', () => {
      const markdown = 'Link to [[Other Note]]';
      const wikilinks = [{
        original: '[[Other Note]]',
        display: 'Other Note',
        exists: true,
        file_id: '123'
      }];

      const html = renderStructuredMarkdown(markdown, wikilinks, []);

      expect(html).toContain('md-wikilink');
      expect(html).toContain('Other Note');
      expect(html).not.toContain('[[');
      expect(html).not.toContain(']]');
    });

    it('should handle broken wikilinks', () => {
      const markdown = 'Link to [[Missing Note]]';
      const wikilinks = [{
        original: '[[Missing Note]]',
        display: 'Missing Note',
        exists: false,
        file_id: ''
      }];

      const html = renderStructuredMarkdown(markdown, wikilinks, []);

      expect(html).toContain('md-wikilink-broken');
    });

    it('should replace image embeds', () => {
      const markdown = '![[image.png]]';
      const embeds = [{
        type: 'image',
        target: 'image.png',
        exists: true
      }];

      const html = renderStructuredMarkdown(markdown, [], embeds);

      expect(html).toContain('md-embed-image');
      expect(html).toContain('image.png');
    });

    it('should handle missing embeds', () => {
      const markdown = '![[missing.pdf]]';
      const embeds = [{
        type: 'pdf',
        target: 'missing.pdf',
        exists: false
      }];

      const html = renderStructuredMarkdown(markdown, [], embeds);

      expect(html).toContain('md-embed-not-found');
      expect(html).toContain('missing.pdf');
    });
  });

  describe('processTags', () => {
    it('should process tags with counts', () => {
      const tags = [
        { name: 'golang', count: 5 },
        { name: 'testing', count: 3 }
      ];

      const processed = processTags(tags);

      expect(processed).toHaveLength(2);
      expect(processed[0]).toEqual({
        name: 'golang',
        count: 5,
        display: '#golang',
        clickable: true
      });
    });

    it('should handle empty tags', () => {
      const processed = processTags([]);
      expect(processed).toEqual([]);
    });

    it('should handle null tags', () => {
      const processed = processTags(null);
      expect(processed).toEqual([]);
    });
  });

  describe('buildOutline', () => {
    it('should build outline from headings', () => {
      const headings = [
        { id: 'heading-1', text: 'First', level: 1, line: 1 },
        { id: 'heading-2', text: 'Second', level: 2, line: 5 }
      ];

      const outline = buildOutline(headings);

      expect(outline).toHaveLength(2);
      expect(outline[0]).toEqual({
        id: 'heading-1',
        text: 'First',
        level: 1,
        line: 1,
        indentClass: 'outline-level-1'
      });
    });

    it('should handle empty headings', () => {
      const outline = buildOutline([]);
      expect(outline).toEqual([]);
    });
  });

  describe('formatReadingTime', () => {
    it('should format zero minutes', () => {
      expect(formatReadingTime(0)).toBe('Less than a minute');
    });

    it('should format one minute', () => {
      expect(formatReadingTime(1)).toBe('1 minute');
    });

    it('should format multiple minutes', () => {
      expect(formatReadingTime(5)).toBe('5 minutes');
    });

    it('should handle null', () => {
      expect(formatReadingTime(null)).toBe('Less than a minute');
    });
  });

  describe('formatStats', () => {
    it('should format complete stats', () => {
      const stats = {
        words: 1500,
        characters: 8000,
        reading_time_minutes: 8
      };

      const formatted = formatStats(stats);

      expect(formatted.words).toBe(1500);
      expect(formatted.characters).toBe(8000);
      expect(formatted.readingTime).toBe('8 minutes');
    });

    it('should handle null stats', () => {
      const formatted = formatStats(null);

      expect(formatted.words).toBe(0);
      expect(formatted.characters).toBe(0);
      expect(formatted.readingTime).toBe('Less than a minute');
    });

    it('should handle missing fields', () => {
      const stats = {
        words: 100
        // missing characters and reading_time_minutes
      };

      const formatted = formatStats(stats);

      expect(formatted.words).toBe(100);
      expect(formatted.characters).toBe(0);
    });
  });

  describe('processBacklinks', () => {
    it('should process backlinks', () => {
      const backlinks = [
        {
          file_id: '123',
          file_name: 'Note 1',
          file_path: '/notes/note1.md',
          context: 'Some context'
        }
      ];

      const processed = processBacklinks(backlinks);

      expect(processed).toHaveLength(1);
      expect(processed[0]).toEqual({
        id: '123',
        name: 'Note 1',
        path: '/notes/note1.md',
        context: 'Some context',
        clickable: true
      });
    });

    it('should handle empty backlinks', () => {
      const processed = processBacklinks([]);
      expect(processed).toEqual([]);
    });

    it('should handle missing context', () => {
      const backlinks = [{
        file_id: '123',
        file_name: 'Note 1',
        file_path: '/notes/note1.md'
      }];

      const processed = processBacklinks(backlinks);

      expect(processed[0].context).toBe('');
    });
  });

  describe('wikilink edge cases', () => {
    it('should handle wikilinks with display text', () => {
      const markdown = '[[Target|Display Text]]';
      const wikilinks = [{
        original: '[[Target|Display Text]]',
        target: 'Target',
        display: 'Display Text',
        exists: true,
        file_id: '456'
      }];

      const html = renderStructuredMarkdown(markdown, wikilinks, []);

      expect(html).toContain('Display Text');
      expect(html).not.toContain('Target');
    });

    it('should handle multiple wikilinks', () => {
      const markdown = '[[Note 1]] and [[Note 2]]';
      const wikilinks = [
        {
          original: '[[Note 1]]',
          display: 'Note 1',
          exists: true,
          file_id: '1'
        },
        {
          original: '[[Note 2]]',
          display: 'Note 2',
          exists: true,
          file_id: '2'
        }
      ];

      const html = renderStructuredMarkdown(markdown, wikilinks, []);

      expect(html).toContain('Note 1');
      expect(html).toContain('Note 2');
    });
  });

  describe('embed edge cases', () => {
    it('should handle note embeds', () => {
      const markdown = '![[Other Note]]';
      const embeds = [{
        type: 'note',
        target: 'Other Note',
        exists: true
      }];

      const html = renderStructuredMarkdown(markdown, [], embeds);

      expect(html).toContain('md-embed-note');
      expect(html).toContain('Other Note');
    });

    it('should handle pdf embeds', () => {
      const markdown = '![[document.pdf]]';
      const embeds = [{
        type: 'pdf',
        target: 'document.pdf',
        exists: true
      }];

      const html = renderStructuredMarkdown(markdown, [], embeds);

      expect(html).toContain('md-embed-pdf');
    });
  });

  describe('security', () => {
    it('should escape HTML in wikilink display text', () => {
      const markdown = '[[Note]]';
      const wikilinks = [{
        original: '[[Note]]',
        display: '<script>alert("xss")</script>',
        exists: true,
        file_id: '123'
      }];

      const html = renderStructuredMarkdown(markdown, wikilinks, []);

      expect(html).not.toContain('<script>');
      expect(html).toContain('&lt;script&gt;');
    });

    it('should escape HTML in embed targets', () => {
      const markdown = '![[<img src=x onerror=alert(1)>]]';
      const embeds = [{
        type: 'note',
        target: '<img src=x onerror=alert(1)>',
        exists: false
      }];

      const html = renderStructuredMarkdown(markdown, [], embeds);

      // Should not contain executable script
      expect(html).not.toContain('<img src=x onerror=');
      // Should contain escaped version
      expect(html).toContain('&lt;img');
    });
  });
});
