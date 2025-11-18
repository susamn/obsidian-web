import axios from 'axios';

const BASE_URL = (import.meta.env.VITE_API_BASE_URL || '/api/v1') + '/files';

export default {
  /**
   * Fetches the directory tree for a given vault and path.
   * @param {string} vaultId - The ID of the vault.
   * @param {string} [path=''] - The directory path (empty for root).
   * @returns {Promise<object>} - A promise that resolves to the directory tree data.
   */
  getTree(vaultId, path = '') {
    return axios.get(`${BASE_URL}/tree/${vaultId}`, { params: { path } })
      .then(response => {
        // API returns: { data: { metadata, children, loaded } }
        // We want to return the children array
        if (response.data && response.data.data && response.data.data.children) {
          return response.data.data.children;
        }
        return [];
      });
  },

  /**
   * Fetches the direct children of a directory for a given vault and path.
   * @param {string} vaultId - The ID of the vault.
   * @param {string} [path=''] - The directory path (empty for root).
   * @returns {Promise<Array<object>>} - A promise that resolves to an array of child nodes.
   */
  getChildren(vaultId, path = '') {
    return axios.get(`${BASE_URL}/children/${vaultId}`, { params: { path } })
      .then(response => {
        // API returns: { data: { path, children, count } }
        if (response.data && response.data.data && response.data.data.children) {
          return response.data.data.children;
        }
        return [];
      });
  },

  /**
   * Fetches metadata for a file or directory.
   * @param {string} vaultId - The ID of the vault.
   * @param {string} path - The file or directory path.
   * @returns {Promise<object>} - A promise that resolves to the metadata.
   */
  getMetadata(vaultId, path) {
    return axios.get(`${BASE_URL}/meta/${vaultId}`, { params: { path } })
      .then(response => {
        // API returns: { data: { path, name, type, ... } }
        if (response.data && response.data.data) {
          return response.data.data;
        }
        return null;
      });
  },

  /**
   * Manually refreshes the cached directory tree for a path.
   * @param {string} vaultId - The ID of the vault.
   * @param {string} [path=''] - The directory path (empty for root).
   * @returns {Promise<object>} - A promise that resolves to a success message.
   */
  refreshTree(vaultId, path = '') {
    return axios.post(`${BASE_URL}/refresh/${vaultId}`, null, { params: { path } });
  },
  /**
   * Fetches the content of a file.
   * @param {string} vaultId - The ID of the vault.
   * @param {string} path - The file path.
   * @returns {Promise<object>} - A promise that resolves to the file content.
   */
  getFileContent(vaultId, path) {
    return axios.get(`${BASE_URL.replace('/files', '')}/files/${vaultId}/${path}`)
      .then(response => {
        // API returns: { content: "...", path: "...", size: 123 }
        return response.data;
      });
  },
};
