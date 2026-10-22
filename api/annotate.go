package api

// annotateStreams 批量标注流归属：后端实现 StreamAnnotator 时一次查询完成，
// 否则回退到逐流 GetStreamService/GetStreamPipe（保持对旧 Backend 实现的兼容）。
// 返回 index -> 标注；indexes 为空时返回 nil。
func annotateStreams(backend Backend, network string, indexes []int64) map[int64]StreamAnnotation {
	if len(indexes) == 0 {
		return nil
	}
	if annotator, ok := backend.(StreamAnnotator); ok {
		return annotator.AnnotateStreams(network, indexes)
	}
	out := make(map[int64]StreamAnnotation, len(indexes))
	for _, index := range indexes {
		out[index] = StreamAnnotation{
			ServiceName: backend.GetStreamService(network, index),
			PipeID:      backend.GetStreamPipe(network, index),
		}
	}
	return out
}
