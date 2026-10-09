import 'package:freezed_annotation/freezed_annotation.dart';

part 'shelf.freezed.dart';
part 'shelf.g.dart';

@freezed
class Shelf with _$Shelf {
  const factory Shelf({
    required String id,
    required String label,
    @Default(10) int capacity,
    @JsonKey(name: 'room_name') String? roomName,
    required List<Note> notes,
  }) = _Shelf;

  factory Shelf.fromJson(Map<String, dynamic> json) => _$ShelfFromJson(json);
}
