import 'package:flutter/material.dart';

import '../auth/session.dart';
import '../chat/chat_api.dart';
import '../chat/models.dart';

/// Manage App model picker: add from Slave Cursor catalog or by id; remove.
class ModelsPage extends StatefulWidget {
  const ModelsPage({super.key, required this.session, this.api});

  final Session session;
  final ChatApi? api;

  @override
  State<ModelsPage> createState() => _ModelsPageState();
}

class _ModelsPageState extends State<ModelsPage> {
  late final ChatApi _api = widget.api ?? ChatApi(session: widget.session);

  ModelManageView? _view;
  String? _error;
  bool _loading = true;
  bool _busy = false;
  final _manualId = TextEditingController();
  final _manualLabel = TextEditingController();

  @override
  void initState() {
    super.initState();
    _load();
  }

  @override
  void dispose() {
    _manualId.dispose();
    _manualLabel.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final v = await _api.manageModels();
      if (!mounted) return;
      setState(() {
        _view = v;
        _loading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _loading = false;
        _error = e.toString();
      });
    }
  }

  Future<void> _run(Future<ModelManageView> Function() op) async {
    if (_busy) return;
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      final v = await op();
      if (!mounted) return;
      setState(() {
        _view = v;
        _busy = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _busy = false;
        _error = e.toString();
      });
    }
  }

  Future<void> _refreshFromSlave() async {
    await _run(() async {
      await _api.refreshModels();
      // Slave report is async over WS; reload manage shortly after.
      await Future<void>.delayed(const Duration(milliseconds: 800));
      return _api.manageModels();
    });
  }

  Set<String> get _selectedIds =>
      _view?.selected.map((m) => m.id).toSet() ?? {};

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('模型列表'),
        actions: [
          IconButton(
            tooltip: '从 Slave 拉取 Cursor 目录',
            onPressed: _busy ? null : _refreshFromSlave,
            icon: const Icon(Icons.cloud_download_outlined),
          ),
          IconButton(
            tooltip: '刷新',
            onPressed: _busy ? null : _load,
            icon: const Icon(Icons.refresh),
          ),
        ],
      ),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : ListView(
              padding: const EdgeInsets.all(16),
              children: [
                if (_error != null)
                  Padding(
                    padding: const EdgeInsets.only(bottom: 12),
                    child: Text(
                      _error!,
                      style: TextStyle(color: Theme.of(context).colorScheme.error),
                    ),
                  ),
                Text(
                  '选择列表（聊天 / 工作流下拉）',
                  style: Theme.of(context).textTheme.titleMedium,
                ),
                const SizedBox(height: 4),
                Text(
                  'Auto 不可删除。Pro 账号可手动加入 composer-2.5 等 id。',
                  style: Theme.of(context).textTheme.bodySmall,
                ),
                const SizedBox(height: 8),
                if (_view == null || _view!.selected.isEmpty)
                  const ListTile(title: Text('（空）'))
                else
                  ..._view!.selected.map((m) {
                    final locked = m.id == 'auto';
                    return ListTile(
                      title: Text(m.label),
                      subtitle: Text(m.id),
                      trailing: locked
                          ? const Chip(label: Text('固定'))
                          : IconButton(
                              tooltip: '移出选择列表',
                              onPressed: _busy
                                  ? null
                                  : () => _run(() => _api.removeModel(m.id)),
                              icon: const Icon(Icons.remove_circle_outline),
                            ),
                    );
                  }),
                const Divider(height: 32),
                Text(
                  'Slave 可用目录（Cursor.models.list）',
                  style: Theme.of(context).textTheme.titleMedium,
                ),
                const SizedBox(height: 4),
                Text(
                  '点 + 加入选择列表。若为空，先确保 Slave 在线并点云下载。',
                  style: Theme.of(context).textTheme.bodySmall,
                ),
                const SizedBox(height: 8),
                if (_view == null || _view!.available.isEmpty)
                  const ListTile(
                    title: Text('暂无数据'),
                    subtitle: Text('重启 Slave 或点右上角从 Slave 拉取'),
                  )
                else
                  ..._view!.available.map((m) {
                    final inSelected = _selectedIds.contains(m.id);
                    return ListTile(
                      title: Text(m.label),
                      subtitle: Text(m.id),
                      trailing: inSelected
                          ? const Chip(label: Text('已选'))
                          : IconButton(
                              tooltip: '加入选择列表',
                              onPressed: _busy
                                  ? null
                                  : () => _run(
                                        () => _api.addModel(
                                          id: m.id,
                                          label: m.label,
                                        ),
                                      ),
                              icon: const Icon(Icons.add_circle_outline),
                            ),
                    );
                  }),
                const Divider(height: 32),
                Text(
                  '手动添加 model id',
                  style: Theme.of(context).textTheme.titleMedium,
                ),
                const SizedBox(height: 8),
                TextField(
                  controller: _manualId,
                  decoration: const InputDecoration(
                    labelText: 'id（如 composer-2.5）',
                    border: OutlineInputBorder(),
                  ),
                ),
                const SizedBox(height: 8),
                TextField(
                  controller: _manualLabel,
                  decoration: const InputDecoration(
                    labelText: '显示名（可选）',
                    border: OutlineInputBorder(),
                  ),
                ),
                const SizedBox(height: 12),
                FilledButton.icon(
                  onPressed: _busy
                      ? null
                      : () {
                          final id = _manualId.text.trim();
                          if (id.isEmpty) return;
                          _run(() async {
                            final v = await _api.addModel(
                              id: id,
                              label: _manualLabel.text.trim().isEmpty
                                  ? null
                                  : _manualLabel.text.trim(),
                            );
                            _manualId.clear();
                            _manualLabel.clear();
                            return v;
                          });
                        },
                  icon: const Icon(Icons.add),
                  label: const Text('加入选择列表'),
                ),
              ],
            ),
    );
  }
}
