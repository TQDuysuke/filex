import os

path = r'packages/core/src/components/public/PeopleSelector.vue'
with open(path, 'r', encoding='utf-8') as f:
    c = f.read()

c = c.replace('Gương mặt nhận diện', 'Recognized People')
c = c.replace('Xem tất cả ảnh', 'View all photos')
c = c.replace('Cuộn trái', 'Scroll left')
c = c.replace('Cuộn phải', 'Scroll right')
c = c.replace('>Tất cả<', '>All<')
c = c.replace('alt="Gương mặt nhận diện"', 'alt="Recognized Person"')
c = c.replace('{{ person.photo_count }} ảnh', "{{ person.photo_count }} {{ person.photo_count === 1 ? 'photo' : 'photos' }}")

with open(path, 'w', encoding='utf-8') as f:
    f.write(c)

print('PeopleSelector.vue updated successfully.')
